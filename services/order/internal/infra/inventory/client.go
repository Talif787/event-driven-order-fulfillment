// Package inventory is the Order service's synchronous client to the Inventory
// service. It adapts the Inventory REST API to the saga.InventoryClient port and
// classifies responses into business declines (cancel the saga) and transient
// failures (retry the saga).
package inventory

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
	"github.com/orderfulfillment/order/internal/infra/tracing"
)

// Client calls the Inventory service over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a client for the given base URL (for example
// http://inventory-api:8080). The timeout bounds each call.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

type reserveLineDTO struct {
	SKU      string `json:"sku"`
	Quantity int32  `json:"quantity"`
}

type reserveRequest struct {
	OrderID string           `json:"orderId"`
	Lines   []reserveLineDTO `json:"lines"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Reserve holds stock for an order. A business decline (insufficient stock,
// unknown SKU, or any other permanent 4xx) is reported as
// saga.ErrReservationRejected; concurrency conflicts and 5xx or transport
// errors are returned as plain errors for the saga to retry.
func (c *Client) Reserve(ctx context.Context, orderID string, lines []saga.ReserveLine) error {
	body := reserveRequest{OrderID: orderID, Lines: make([]reserveLineDTO, 0, len(lines))}
	for _, l := range lines {
		body.Lines = append(body.Lines, reserveLineDTO{SKU: l.SKU, Quantity: l.Quantity})
	}
	status, env, err := c.do(ctx, http.MethodPost, "/v1/reservations", body)
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusConflict && env.Error.Code == "CONFLICT":
		return fmt.Errorf("inventory concurrency conflict: %s", env.Error.Message)
	case status >= 400 && status < 500:
		return fmt.Errorf("%w: %s", saga.ErrReservationRejected, env.Error.Message)
	default:
		return fmt.Errorf("inventory reserve failed: status %d: %s", status, env.Error.Message)
	}
}

// Release returns held stock to available (compensation). It is idempotent on
// the Inventory side.
func (c *Client) Release(ctx context.Context, orderID string) error {
	return c.transition(ctx, orderID, "release")
}

// Commit finalizes held stock. It is idempotent on the Inventory side.
func (c *Client) Commit(ctx context.Context, orderID string) error {
	return c.transition(ctx, orderID, "commit")
}

func (c *Client) transition(ctx context.Context, orderID, action string) error {
	path := "/v1/reservations/" + orderID + "/" + action
	status, env, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusConflict && env.Error.Code == "INVALID_STATE":
		// The reservation is already in the target state (a retried compensation
		// or commit after a crash). The effect is achieved, so this is success.
		return nil
	default:
		return fmt.Errorf("inventory %s failed: status %d: %s", action, status, env.Error.Message)
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (int, errorEnvelope, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, errorEnvelope{}, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, errorEnvelope{}, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	tracing.InjectToHTTP(ctx, req.Header)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, errorEnvelope{}, fmt.Errorf("call inventory %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, errorEnvelope{}, fmt.Errorf("read inventory response: %w", err)
	}
	var env errorEnvelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env) // best-effort; success bodies simply have no error field
	}
	return resp.StatusCode, env, nil
}
