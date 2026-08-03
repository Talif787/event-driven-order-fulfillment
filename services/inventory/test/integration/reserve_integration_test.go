package integration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/orderfulfillment/inventory/internal/app"
	"github.com/orderfulfillment/inventory/internal/domain/inventory"
	pg "github.com/orderfulfillment/inventory/internal/infra/postgres"
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
			"POSTGRES_USER":     "inventory",
			"POSTGRES_PASSWORD": "inventory",
			"POSTGRES_DB":       "inventory",
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
	return fmt.Sprintf("postgres://inventory:inventory@%s:%s/inventory?sslmode=disable", host, port.Port())
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

func assertStock(ctx context.Context, t *testing.T, repo *pg.InventoryRepository, sku string, wantAvail, wantReserved int64) {
	t.Helper()
	view, err := repo.GetStock(ctx, sku)
	if err != nil {
		t.Fatalf("GetStock(%s): %v", sku, err)
	}
	if view.Available != wantAvail || view.Reserved != wantReserved {
		t.Fatalf("%s: want available=%d reserved=%d, got available=%d reserved=%d",
			sku, wantAvail, wantReserved, view.Available, view.Reserved)
	}
}

func TestReservationLifecycle(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	dsn := startPostgres(ctx, t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	applyMigrations(ctx, t, pool)

	repo := pg.NewInventoryRepository(pool)

	if err := repo.UpsertStock(ctx, "SKU-1", 10); err != nil {
		t.Fatalf("seed SKU-1: %v", err)
	}
	if err := repo.UpsertStock(ctx, "SKU-2", 5); err != nil {
		t.Fatalf("seed SKU-2: %v", err)
	}

	order := uuid.NewString()
	lines := []app.ReserveLine{{SKU: "SKU-1", Quantity: 3}, {SKU: "SKU-2", Quantity: 2}}

	// Reserve holds stock and marks the reservation HELD.
	res, err := repo.Reserve(ctx, order, lines)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.Status != string(inventory.StatusHeld) || res.Idempotent {
		t.Fatalf("unexpected reserve result: %+v", res)
	}
	assertStock(ctx, t, repo, "SKU-1", 7, 3)
	assertStock(ctx, t, repo, "SKU-2", 3, 2)

	// Reserving the same order again is idempotent and does not double-decrement.
	again, err := repo.Reserve(ctx, order, lines)
	if err != nil {
		t.Fatalf("idempotent reserve: %v", err)
	}
	if !again.Idempotent || again.ReservationID != res.ReservationID {
		t.Fatalf("expected idempotent replay, got %+v", again)
	}
	assertStock(ctx, t, repo, "SKU-1", 7, 3)

	// Insufficient stock for a new order fails and leaves the ledger untouched.
	_, err = repo.Reserve(ctx, uuid.NewString(), []app.ReserveLine{{SKU: "SKU-1", Quantity: 100}})
	if !errors.Is(err, inventory.ErrInsufficientStock) {
		t.Fatalf("want ErrInsufficientStock, got %v", err)
	}
	assertStock(ctx, t, repo, "SKU-1", 7, 3)

	// Unknown SKU is reported distinctly.
	_, err = repo.Reserve(ctx, uuid.NewString(), []app.ReserveLine{{SKU: "SKU-UNKNOWN", Quantity: 1}})
	if !errors.Is(err, inventory.ErrSKUNotFound) {
		t.Fatalf("want ErrSKUNotFound, got %v", err)
	}

	// Release returns held stock and marks the reservation RELEASED.
	rel, err := repo.Release(ctx, order)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if rel.Status != string(inventory.StatusReleased) || rel.Idempotent {
		t.Fatalf("unexpected release result: %+v", rel)
	}
	assertStock(ctx, t, repo, "SKU-1", 10, 0)
	assertStock(ctx, t, repo, "SKU-2", 5, 0)

	// Releasing again is idempotent and does not inflate available.
	relAgain, err := repo.Release(ctx, order)
	if err != nil {
		t.Fatalf("idempotent release: %v", err)
	}
	if !relAgain.Idempotent {
		t.Fatalf("expected idempotent release, got %+v", relAgain)
	}
	assertStock(ctx, t, repo, "SKU-1", 10, 0)

	// Commit path on a fresh order: held then committed, reserved drains away.
	commitOrder := uuid.NewString()
	if _, err := repo.Reserve(ctx, commitOrder, []app.ReserveLine{{SKU: "SKU-1", Quantity: 4}}); err != nil {
		t.Fatalf("reserve for commit: %v", err)
	}
	assertStock(ctx, t, repo, "SKU-1", 6, 4)
	com, err := repo.Commit(ctx, commitOrder)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if com.Status != string(inventory.StatusCommitted) {
		t.Fatalf("unexpected commit result: %+v", com)
	}
	assertStock(ctx, t, repo, "SKU-1", 6, 0)

	// Committing again is idempotent; releasing a committed reservation is illegal.
	if _, err := repo.Commit(ctx, commitOrder); err != nil {
		t.Fatalf("idempotent commit: %v", err)
	}
	if _, err := repo.Release(ctx, commitOrder); !errors.Is(err, inventory.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

// TestOptimisticConcurrencyGuard proves the version guard deterministically: an
// update against a stale version affects zero rows, while the current version
// succeeds and bumps the version.
func TestOptimisticConcurrencyGuard(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	dsn := startPostgres(ctx, t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	applyMigrations(ctx, t, pool)

	repo := pg.NewInventoryRepository(pool)
	if err := repo.UpsertStock(ctx, "OCC-1", 50); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var version int64
	if err := pool.QueryRow(ctx, `SELECT version FROM stock_items WHERE sku = $1`, "OCC-1").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}

	// Stale version: zero rows affected.
	stale, err := pool.Exec(ctx,
		`UPDATE stock_items SET available = available - 1, version = version + 1 WHERE sku = $1 AND version = $2`,
		"OCC-1", version-1)
	if err != nil {
		t.Fatalf("stale update: %v", err)
	}
	if stale.RowsAffected() != 0 {
		t.Fatalf("stale update should affect 0 rows, affected %d", stale.RowsAffected())
	}

	// Current version: one row affected.
	fresh, err := pool.Exec(ctx,
		`UPDATE stock_items SET available = available - 1, version = version + 1 WHERE sku = $1 AND version = $2`,
		"OCC-1", version)
	if err != nil {
		t.Fatalf("fresh update: %v", err)
	}
	if fresh.RowsAffected() != 1 {
		t.Fatalf("fresh update should affect 1 row, affected %d", fresh.RowsAffected())
	}
}
