package relay

import (
	"context"
	"log/slog"
	"time"

	"github.com/orderfulfillment/order/internal/app"
)

// Store drains a batch of outbox rows, publishing them via the supplied func
// inside its own transaction.
type Store interface {
	DrainBatch(ctx context.Context, batchSize int, publish func(context.Context, []app.OutboxRecord) error) (int, error)
}

// Worker continuously relays the transactional outbox to the event backbone.
type Worker struct {
	store     Store
	publisher app.Publisher
	interval  time.Duration
	batchSize int
	logger    *slog.Logger
}

func NewWorker(store Store, publisher app.Publisher, interval time.Duration, batchSize int, logger *slog.Logger) *Worker {
	return &Worker{store: store, publisher: publisher, interval: interval, batchSize: batchSize, logger: logger}
}

// Run drains the outbox until the context is cancelled. A full batch triggers an
// immediate re-drain so backlog clears quickly; an empty or partial cycle waits
// one interval so an idle relay is cheap.
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("outbox relay started",
		slog.Int("batch_size", w.batchSize),
		slog.Duration("poll_interval", w.interval),
	)
	for {
		if ctx.Err() != nil {
			return nil
		}
		processed, err := w.store.DrainBatch(ctx, w.batchSize, w.publisher.Publish)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("relay cycle failed", slog.String("error", err.Error()))
		case processed == w.batchSize:
			continue
		case processed > 0:
			w.logger.Info("relay published batch", slog.Int("count", processed))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.interval):
		}
	}
}
