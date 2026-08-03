package integration

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/orderfulfillment/order/internal/app/projection"
	"github.com/orderfulfillment/order/internal/contracts"
	pg "github.com/orderfulfillment/order/internal/infra/postgres"
)

func TestProjection_AppliesOrderPlacedIdempotently(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	applyMigrations(ctx, t, pool)

	repo := pg.NewProjectionRepository(pool)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := projection.NewService(repo, logger, noop.NewTracerProvider().Tracer("test"))

	orderID := "33333333-3333-3333-3333-333333333333"
	event := contracts.OrderPlacedV1{
		OrderID:    orderID,
		CustomerID: "44444444-4444-4444-4444-444444444444",
		Items:      []contracts.LineItem{{SKU: "SKU-1", Quantity: 2, UnitPriceMinor: 1500}},
		ShipTo:     contracts.Address{Line1: "1 Main St", City: "Boston", Region: "MA", PostalCode: "02118", Country: "US"},
		TotalMinor: 3000,
		Currency:   "USD",
		PlacedAt:   contracts.FormatTime(time.Now()),
	}
	payload, err := event.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := svc.Apply(ctx, contracts.TypeOrderPlaced, payload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := svc.Apply(ctx, contracts.TypeOrderPlaced, payload); err != nil {
		t.Fatalf("re-apply: %v", err)
	}

	view, err := repo.GetOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("get projection: %v", err)
	}
	if view.Status != "PENDING" || view.TotalMinor != 3000 || view.Currency != "USD" || view.Version != 1 {
		t.Fatalf("unexpected projection: %+v", view)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_projections WHERE order_id = $1`, orderID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("projection rows = %d, want 1 (idempotent upsert)", count)
	}
}
