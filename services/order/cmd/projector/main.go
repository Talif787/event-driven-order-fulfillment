package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/orderfulfillment/order/internal/app/projection"
	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/logging"
	"github.com/orderfulfillment/order/internal/infra/metrics"
	"github.com/orderfulfillment/order/internal/infra/postgres"
	"github.com/orderfulfillment/order/internal/infra/telemetry"
	"github.com/orderfulfillment/order/internal/worker/projector"
)

func main() {
	if err := run(); err != nil {
		slog.Error("projector startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(cfg.ServiceName+"-projector", cfg.Environment, os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	metricsSrv := metrics.StartServer(cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	tel, err := telemetry.Setup(ctx, cfg.ServiceName+"-projector", cfg.Environment, cfg.Telemetry.OTLPEndpoint, cfg.Telemetry.SampleRatio)
	if err != nil {
		return err
	}
	defer func() { _ = tel.Shutdown(context.Background()) }()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	service := projection.NewService(postgres.NewProjectionRepository(pool), logger, tel.Tracer())
	worker := projector.NewWorker(cfg.Kafka.Brokers, cfg.Projector.Topics, cfg.Projector.GroupID, service, logger)
	defer func() { _ = worker.Close() }()

	logger.Info("projector connecting", slog.Any("brokers", cfg.Kafka.Brokers), slog.Any("topics", cfg.Projector.Topics))
	return worker.Run(ctx)
}
