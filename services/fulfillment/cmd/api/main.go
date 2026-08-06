package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/orderfulfillment/fulfillment/internal/app"
	"github.com/orderfulfillment/fulfillment/internal/infra/config"
	infrakafka "github.com/orderfulfillment/fulfillment/internal/infra/kafka"
	"github.com/orderfulfillment/fulfillment/internal/infra/logging"
	"github.com/orderfulfillment/fulfillment/internal/infra/postgres"
	"github.com/orderfulfillment/fulfillment/internal/infra/telemetry"
	httppres "github.com/orderfulfillment/fulfillment/internal/presentation/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fulfillment api exited", slog.String("error", err.Error()))
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

	publisher := infrakafka.NewPublisher(cfg.Kafka.Brokers, cfg.Kafka.EventsTopic)
	defer func() { _ = publisher.Close() }()

	service := app.NewService(postgres.NewShipmentRepository(pool), publisher, app.SystemClock{}, logger)
	handlers := httppres.NewHandlers(service, logger)
	health := httppres.NewHealthHandlers(pool)
	router := httppres.NewRouter(handlers, health, cfg.Auth, cfg.CORSAllowedOrigins, logger)
	srv := httppres.NewServer(cfg, router)

	logger.Info("fulfillment api starting", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Environment))
	return httppres.Serve(ctx, srv, cfg.Timeouts)
}
