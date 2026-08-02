package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/orderfulfillment/order/internal/app/command"
	"github.com/orderfulfillment/order/internal/app/query"
	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/logging"
	"github.com/orderfulfillment/order/internal/infra/postgres"
	"github.com/orderfulfillment/order/internal/infra/system"
	"github.com/orderfulfillment/order/internal/infra/telemetry"
	httpapi "github.com/orderfulfillment/order/internal/presentation/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", slog.String("error", err.Error()))
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

	repo := postgres.NewOrderRepository(pool)
	idem := postgres.NewIdempotencyStore(pool)

	placeHandler := command.NewPlaceOrderHandler(repo, idem, system.IDGenerator{}, system.Clock{}, logger, tel.Tracer())
	getHandler := query.NewGetOrderHandler(repo, tel.Tracer())

	handlers := httpapi.NewHandlers(placeHandler, getHandler, logger)
	health := httpapi.NewHealthHandlers(pool)
	router := httpapi.NewRouter(handlers, health, cfg.Auth, logger)
	server := httpapi.NewServer(cfg, router)

	logger.Info("order service listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Environment))
	return httpapi.Serve(ctx, server, cfg.Timeouts)
}
