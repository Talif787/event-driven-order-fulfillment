package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/orderfulfillment/fulfillment/internal/infra/config"
)

// NewServer builds an http.Server with hardened timeouts.
func NewServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Timeouts.ReadHeader,
		ReadTimeout:       cfg.Timeouts.Read,
		WriteTimeout:      cfg.Timeouts.Write,
		IdleTimeout:       cfg.Timeouts.Idle,
	}
}

// Serve runs the server until the context is cancelled, then drains gracefully.
func Serve(ctx context.Context, srv *http.Server, timeouts config.TimeoutConfig) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeouts.Shutdown)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
