package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/orderfulfillment/order/internal/app/saga"
)

// Client calls the Payment service over HTTP. It implements saga.PaymentGateway
// and replaces the stub once a payment service URL is configured. The capture
// endpoint is idempotent on order id, so retrying it never double charges.
//
// A transient failure (transport error, 5xx, or 429) is retried in place with a
// short backoff, so a briefly unreachable or still-booting payment service is
// absorbed by this step rather than surfacing as an error that would tear down
// the saga worker. A clean approve or decline (any 2xx) is terminal and returns
// immediately, and a 4xx is a caller error that is not retried. Only a failure
// that outlives the retry budget propagates, and the saga treats that as
// transient and reprocesses the event.
type Client struct {
	baseURL     string
	http        *http.Client
	maxAttempts int
	backoff     time.Duration
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		http:        &http.Client{Timeout: timeout},
		maxAttempts: 4,
		backoff:     250 * time.Millisecond,
	}
}

type captureRequest struct {
	OrderID     string `json:"orderId"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

type captureResponse struct {
	PaymentID         string `json:"paymentId"`
	OrderID           string `json:"orderId"`
	Status            string `json:"status"`
	ProviderReference string `json:"providerReference"`
	Approved          bool   `json:"approved"`
	DeclineReason     string `json:"declineReason"`
}

// Authorize charges the order, retrying transient failures in place. A clean
// approve or decline is returned as a PaymentResult with a nil error; only a
// failure that persists past the retry budget is returned as an error.
func (c *Client) Authorize(ctx context.Context, req saga.PaymentRequest) (saga.PaymentResult, error) {
	raw, err := json.Marshal(captureRequest{
		OrderID: req.OrderID, AmountMinor: req.AmountMinor, Currency: req.Currency,
	})
	if err != nil {
		return saga.PaymentResult{}, fmt.Errorf("marshal payment request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		result, retryable, err := c.authorizeOnce(ctx, raw)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retryable {
			return saga.PaymentResult{}, err
		}
		if attempt == c.maxAttempts {
			break
		}
		// Back off before the next attempt, but abort promptly if the caller
		// cancels the context.
		select {
		case <-ctx.Done():
			return saga.PaymentResult{}, ctx.Err()
		case <-time.After(c.backoff * time.Duration(attempt)):
		}
	}
	return saga.PaymentResult{}, fmt.Errorf("payment service unavailable after %d attempts: %w", c.maxAttempts, lastErr)
}

// authorizeOnce performs a single call. The bool reports whether a failure is
// transient (transport error, 5xx, or 429) and therefore worth retrying. Any
// 2xx response, including a clean decline, is terminal and returns retryable
// false; a 4xx is a caller error and is also not retried.
func (c *Client) authorizeOnce(ctx context.Context, raw []byte) (saga.PaymentResult, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/payments", bytes.NewReader(raw))
	if err != nil {
		return saga.PaymentResult{}, false, fmt.Errorf("build payment request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// Connection refused, reset, or timeout: the service is unreachable now
		// but may be reachable on the next attempt.
		return saga.PaymentResult{}, true, fmt.Errorf("call payment service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return saga.PaymentResult{}, true, fmt.Errorf("read payment response: %w", err)
	}

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return saga.PaymentResult{}, true, fmt.Errorf("payment service status %d: %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return saga.PaymentResult{}, false, fmt.Errorf("payment service status %d: %s", resp.StatusCode, string(data))
	}

	var out captureResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return saga.PaymentResult{}, false, fmt.Errorf("decode payment response: %w", err)
	}
	if out.Approved {
		return saga.PaymentResult{Approved: true, Reference: out.ProviderReference}, false, nil
	}
	return saga.PaymentResult{Approved: false, Decline: out.DeclineReason}, false, nil
}
