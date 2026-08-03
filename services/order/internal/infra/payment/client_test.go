package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/orderfulfillment/order/internal/app/saga"
)

func testClient(url string) *Client {
	c := NewClient(url, 2*time.Second)
	c.backoff = time.Millisecond // keep tests fast
	return c
}

// A transient failure (503) is retried in place, and the subsequent success is
// returned. The order is charged exactly once from the caller's point of view.
func TestAuthorizeRetriesTransientThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"warming up"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"CAPTURED","approved":true,"providerReference":"psp_test"}`))
	}))
	defer srv.Close()

	res, err := testClient(srv.URL).Authorize(context.Background(),
		saga.PaymentRequest{OrderID: "o1", AmountMinor: 3000, Currency: "USD"})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if !res.Approved || res.Reference != "psp_test" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (transient then success), got %d", got)
	}
}

// A clean business decline is a 2xx with approved false. It is terminal: the
// saga must compensate, so the client returns no error and does not retry.
func TestAuthorizeDeclineIsTerminalNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"DECLINED","approved":false,"declineReason":"card declined"}`))
	}))
	defer srv.Close()

	res, err := testClient(srv.URL).Authorize(context.Background(),
		saga.PaymentRequest{OrderID: "o1", AmountMinor: 3000, Currency: "USD"})
	if err != nil {
		t.Fatalf("clean decline must not error, got %v", err)
	}
	if res.Approved || res.Decline != "card declined" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("decline must not retry, got %d calls", got)
	}
}

// A failure that outlives the retry budget propagates as an error, having tried
// the full number of attempts.
func TestAuthorizeExhaustsRetriesOnSustainedOutage(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	c.maxAttempts = 3

	if _, err := c.Authorize(context.Background(),
		saga.PaymentRequest{OrderID: "o1", AmountMinor: 3000, Currency: "USD"}); err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

// A 4xx is a caller error, not a transient one: it is returned immediately
// without consuming the retry budget.
func TestAuthorizeDoesNotRetryClientError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR"}}`))
	}))
	defer srv.Close()

	if _, err := testClient(srv.URL).Authorize(context.Background(),
		saga.PaymentRequest{OrderID: "o1", AmountMinor: 3000, Currency: "USD"}); err == nil {
		t.Fatal("expected an error on 4xx")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("4xx must not retry, got %d calls", got)
	}
}
