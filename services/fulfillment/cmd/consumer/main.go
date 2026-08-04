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
	"github.com/orderfulfillment/fulfillment/internal/worker/consumer"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fulfillment consumer exited", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	name := cfg.ServiceName + "-consumer"
	logger := logging.New(name, cfg.Environment, os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.Setup(ctx, name, cfg.Environment, cfg.Telemetry.OTLPEndpoint, cfg.Telemetry.SampleRatio)
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
	worker := consumer.NewWorker(cfg.Kafka.Brokers, cfg.Consumer.Topics, cfg.Consumer.GroupID, service, logger)
	defer func() { _ = worker.Close() }()

	logger.Info("fulfillment consumer connecting",
		slog.Any("brokers", cfg.Kafka.Brokers),
		slog.Any("topics", cfg.Consumer.Topics),
		slog.String("group", cfg.Consumer.GroupID),
	)
	return worker.Run(ctx)
}
