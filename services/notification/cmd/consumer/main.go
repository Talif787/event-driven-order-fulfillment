package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/orderfulfillment/notification/internal/app"
	"github.com/orderfulfillment/notification/internal/infra/config"
	"github.com/orderfulfillment/notification/internal/infra/deadletter"
	"github.com/orderfulfillment/notification/internal/infra/logging"
	"github.com/orderfulfillment/notification/internal/infra/metrics"
	"github.com/orderfulfillment/notification/internal/infra/notifier"
	"github.com/orderfulfillment/notification/internal/infra/postgres"
	"github.com/orderfulfillment/notification/internal/infra/telemetry"
	"github.com/orderfulfillment/notification/internal/worker/consumer"
)

func main() {
	if err := run(); err != nil {
		slog.Error("notification consumer exited", slog.String("error", err.Error()))
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

	metricsSrv := metrics.StartServer(cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	dispatcher := app.NewDispatcher(
		postgres.NewNotificationRepository(pool),
		notifier.NewLogSender(logger),
		app.SystemClock{},
		logger,
	)
	dlq := deadletter.NewPublisher(cfg.Kafka.Brokers, cfg.DeadLetterTopic)
	defer func() { _ = dlq.Close() }()

	worker := consumer.NewWorker(cfg.Kafka.Brokers, cfg.Consumer.Topics, cfg.Consumer.GroupID, dispatcher, logger, tel.Tracer(), dlq)
	defer func() { _ = worker.Close() }()

	logger.Info("notification consumer connecting",
		slog.Any("brokers", cfg.Kafka.Brokers),
		slog.Any("topics", cfg.Consumer.Topics),
		slog.String("group", cfg.Consumer.GroupID),
	)
	return worker.Run(ctx)
}
