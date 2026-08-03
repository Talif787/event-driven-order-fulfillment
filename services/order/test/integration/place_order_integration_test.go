package integration

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/orderfulfillment/order/internal/app/command"
	"github.com/orderfulfillment/order/internal/app/query"
	pg "github.com/orderfulfillment/order/internal/infra/postgres"
	"github.com/orderfulfillment/order/internal/infra/system"
)

// requireIntegration skips unless explicitly enabled, keeping the default
// `go test ./...` run free of any Docker requirement.
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run integration tests (requires Docker)")
	}
}

func startPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "order",
			"POSTGRES_PASSWORD": "order",
			"POSTGRES_DB":       "order",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}
	return fmt.Sprintf("postgres://order:order@%s:%s/order?sslmode=disable", host, port.Port())
}

func applyMigrations(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := fs.ReadDir(pg.MigrationsFS, "migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		data, err := fs.ReadFile(pg.MigrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func newPlaceHandler(pool *pgxpool.Pool) *command.PlaceOrderHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracer := noop.NewTracerProvider().Tracer("integration")
	return command.NewPlaceOrderHandler(
		pg.NewOrderRepository(pool), pg.NewIdempotencyStore(pool),
		system.IDGenerator{}, system.Clock{}, logger, tracer,
	)
}

func sampleCommand(key string) command.PlaceOrderCommand {
	return command.PlaceOrderCommand{
		CustomerID:     "11111111-1111-1111-1111-111111111111",
		IdempotencyKey: key,
		Lines: []command.PlaceOrderLine{
			{SKU: "SKU-1", Quantity: 2, UnitPriceMinor: 1500, Currency: "USD"},
			{SKU: "SKU-2", Quantity: 1, UnitPriceMinor: 500, Currency: "USD"},
		},
		Ship: command.ShipTo{Line1: "1 Main St", City: "Boston", Region: "MA", PostalCode: "02118", Country: "US"},
	}
}

func TestPlaceOrder_PersistsEventAndOutbox(t *testing.T) {
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
	res, err := place.Handle(ctx, sampleCommand("integration-key-0001"))
	if err != nil {
		t.Fatalf("place order: %v", err)
	}

	var eventCount, outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_events WHERE order_id = $1`, res.OrderID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id = $1`, res.OrderID).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if eventCount != 1 || outboxCount != 1 {
		t.Fatalf("expected 1 event and 1 outbox row, got events=%d outbox=%d", eventCount, outboxCount)
	}

	get := query.NewGetOrderHandler(pg.NewProjectionRepository(pool), pg.NewOrderRepository(pool), noop.NewTracerProvider().Tracer("integration"))
	view, err := get.Handle(ctx, res.OrderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if view.Status != "PENDING" || view.TotalMinor != 3500 {
		t.Fatalf("unexpected view: status=%s total=%d", view.Status, view.TotalMinor)
	}
}

func TestPlaceOrder_IdempotentAcrossRequests(t *testing.T) {
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
	first, err := place.Handle(ctx, sampleCommand("integration-key-dup"))
	if err != nil {
		t.Fatalf("first place: %v", err)
	}
	second, err := place.Handle(ctx, sampleCommand("integration-key-dup"))
	if err != nil {
		t.Fatalf("second place: %v", err)
	}
	if first.OrderID != second.OrderID || !second.Idempotent {
		t.Fatalf("expected idempotent replay of same order, got %s vs %s (idempotent=%v)", first.OrderID, second.OrderID, second.Idempotent)
	}

	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_events`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected exactly one persisted order event, got %d", total)
	}
}
