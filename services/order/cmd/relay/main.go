package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/kafka"
	"github.com/orderfulfillment/order/internal/infra/logging"
	"github.com/orderfulfillment/order/internal/infra/postgres"
	"github.com/orderfulfillment/order/internal/worker/relay"
)

func main() {
	if err := run(); err != nil {
		slog.Error("relay startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(cfg.ServiceName+"-relay", cfg.Environment, os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	publisher := kafka.NewPublisher(cfg.Kafka.Brokers)
	defer func() { _ = publisher.Close() }()

	worker := relay.NewWorker(postgres.NewRelayStore(pool), publisher, cfg.Relay.PollInterval, cfg.Relay.BatchSize, logger)
	logger.Info("relay connecting", slog.Any("brokers", cfg.Kafka.Brokers))
	return worker.Run(ctx)
}
