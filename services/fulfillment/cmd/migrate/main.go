package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/orderfulfillment/fulfillment/internal/infra/postgres"
)

type migration struct {
	version string
	name    string
	up      string
	down    string
}

func main() {
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if err := run(direction); err != nil {
		slog.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(direction string) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	switch direction {
	case "up":
		return migrateUp(ctx, conn, migrations)
	case "down":
		return migrateDown(ctx, conn, migrations)
	default:
		return fmt.Errorf("unknown direction %q (use up or down)", direction)
	}
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(postgres.MigrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	byVersion := map[string]*migration{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		version, _, _ := strings.Cut(name, "_")
		content, err := fs.ReadFile(postgres.MigrationsFS, "migrations/"+name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		m := byVersion[version]
		if m == nil {
			m = &migration{version: version, name: name}
			byVersion[version] = m
		}
		if strings.HasSuffix(name, ".up.sql") {
			m.up = string(content)
		} else if strings.HasSuffix(name, ".down.sql") {
			m.down = string(content)
		}
	}
	out := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func migrateUp(ctx context.Context, conn *pgx.Conn, migrations []migration) error {
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyInTx(ctx, conn, m.up, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version)
			return e
		}); err != nil {
			return fmt.Errorf("apply up %s: %w", m.version, err)
		}
		slog.Info("applied migration", slog.String("version", m.version))
	}
	return nil
}

func migrateDown(ctx context.Context, conn *pgx.Conn, migrations []migration) error {
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return err
	}
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		if !applied[m.version] {
			continue
		}
		if err := applyInTx(ctx, conn, m.down, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.version)
			return e
		}); err != nil {
			return fmt.Errorf("apply down %s: %w", m.version, err)
		}
		slog.Info("rolled back migration", slog.String("version", m.version))
		return nil
	}
	return nil
}

func appliedVersions(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied: %w", err)
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyInTx(ctx context.Context, conn *pgx.Conn, sqlText string, record func(pgx.Tx) error) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if strings.TrimSpace(sqlText) != "" {
		if _, err := tx.Exec(ctx, sqlText); err != nil {
			return err
		}
	}
	if err := record(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
