package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/orderfulfillment/inventory/internal/app"
	"github.com/orderfulfillment/inventory/internal/infra/config"
	"github.com/orderfulfillment/inventory/internal/infra/logging"
	"github.com/orderfulfillment/inventory/internal/infra/postgres"
	"github.com/orderfulfillment/inventory/internal/infra/telemetry"
	httppres "github.com/orderfulfillment/inventory/internal/presentation/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("inventory api exited", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(cfg.ServiceName, cfg.Environment, os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.Setup(ctx, cfg.ServiceName, cfg.Environment, cfg.Telemetry.OTLPEndpoint, cfg.Telemetry.SampleRatio)
	if err != nil {
		return err
	}
	defer func() { _ = tel.Shutdown(context.Background()) }()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := postgres.NewInventoryRepository(pool)
	tracer := tel.Tracer()

	handlers := httppres.NewHandlers(
		app.NewReserveStockHandler(repo, logger, tracer),
		app.NewReleaseReservationHandler(repo, logger, tracer),
		app.NewCommitReservationHandler(repo, logger, tracer),
		app.NewGetReservationHandler(repo, tracer),
		app.NewGetStockHandler(repo, tracer),
		app.NewSetStockHandler(repo, logger, tracer),
		logger,
	)
	health := httppres.NewHealthHandlers(pool)
	router := httppres.NewRouter(handlers, health, cfg.Auth, logger)
	srv := httppres.NewServer(cfg, router)

	logger.Info("inventory api starting", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Environment))
	return httppres.Serve(ctx, srv, cfg.Timeouts)
}
