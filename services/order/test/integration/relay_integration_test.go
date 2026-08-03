package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/order/internal/app"
	pg "github.com/orderfulfillment/order/internal/infra/postgres"
)

var errPublish = errors.New("publish failed")

type capturePublisher struct{ records []app.OutboxRecord }

func (p *capturePublisher) Publish(_ context.Context, records []app.OutboxRecord) error {
	p.records = append(p.records, records...)
	return nil
}

func TestRelay_PublishesAndMarksOutbox(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	applyMigrations(ctx, t, pool)

	place := newPlaceHandler(pool)
	res, err := place.Handle(ctx, sampleCommand("relay-key-0001"))
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	pub := &capturePublisher{}
	store := pg.NewRelayStore(pool)

	processed, err := store.DrainBatch(ctx, 100, pub.Publish)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if len(pub.records) != 1 {
		t.Fatalf("published %d records, want 1", len(pub.records))
	}
	rec := pub.records[0]
	if rec.Topic != "orders.events" || rec.EventType != "order.placed.v1" || rec.AggregateID != res.OrderID {
		t.Fatalf("unexpected published record: %+v", rec)
	}
	if rec.EventID == "" {
		t.Fatal("published record missing event id")
	}

	processed, err = store.DrainBatch(ctx, 100, pub.Publish)
	if err != nil {
		t.Fatalf("second drain: %v", err)
	}
	if processed != 0 {
		t.Fatalf("second processed = %d, want 0 (already published)", processed)
	}

	var unpublished int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatalf("count: %v", err)
	}
	if unpublished != 0 {
		t.Fatalf("unpublished = %d, want 0", unpublished)
	}
}

func TestRelay_PublishFailureLeavesRowUnpublished(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	applyMigrations(ctx, t, pool)

	place := newPlaceHandler(pool)
	if _, err := place.Handle(ctx, sampleCommand("relay-key-fail")); err != nil {
		t.Fatalf("place: %v", err)
	}

	store := pg.NewRelayStore(pool)
	failing := func(context.Context, []app.OutboxRecord) error { return errPublish }
	if _, err := store.DrainBatch(ctx, 100, failing); err == nil {
		t.Fatal("expected drain to fail when publish fails")
	}

	var unpublished int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatalf("count: %v", err)
	}
	if unpublished != 1 {
		t.Fatalf("unpublished = %d, want 1 (transaction rolled back)", unpublished)
	}

	ok := &capturePublisher{}
	processed, err := store.DrainBatch(ctx, 100, ok.Publish)
	if err != nil {
		t.Fatalf("retry drain: %v", err)
	}
	if processed != 1 {
		t.Fatalf("retry processed = %d, want 1", processed)
	}
}
