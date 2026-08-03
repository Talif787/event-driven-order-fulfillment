package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/orderfulfillment/order/internal/app/command"
	"github.com/orderfulfillment/order/internal/app/saga"
	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/inventory"
	"github.com/orderfulfillment/order/internal/infra/logging"
	"github.com/orderfulfillment/order/internal/infra/payment"
	"github.com/orderfulfillment/order/internal/infra/postgres"
	"github.com/orderfulfillment/order/internal/infra/system"
	"github.com/orderfulfillment/order/internal/infra/telemetry"
	"github.com/orderfulfillment/order/internal/worker/orchestrator"
)

// orderController adapts the confirm and cancel command handlers to the
// saga.OrderController port.
type orderController struct {
	confirm *command.ConfirmOrderHandler
	cancel  *command.CancelOrderHandler
}

func (c orderController) Confirm(ctx context.Context, orderID string) error {
	return c.confirm.Handle(ctx, orderID)
}

func (c orderController) Cancel(ctx context.Context, orderID, reason string) error {
	return c.cancel.Handle(ctx, orderID, reason)
}

func main() {
	if err := run(); err != nil {
		slog.Error("orchestrator startup failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	name := cfg.ServiceName + "-orchestrator"
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

	tracer := tel.Tracer()
	repo := postgres.NewOrderRepository(pool)
	orders := orderController{
		confirm: command.NewConfirmOrderHandler(repo, system.Clock{}, logger, tracer),
		cancel:  command.NewCancelOrderHandler(repo, system.Clock{}, logger, tracer),
	}
	orch := saga.NewOrchestrator(
		inventory.NewClient(cfg.Saga.InventoryBaseURL, cfg.Saga.HTTPTimeout),
		payment.NewStubGateway(cfg.Saga.PaymentOutcome),
		orders,
		postgres.NewSagaRepository(pool),
		logger,
		tracer,
	)

	worker := orchestrator.NewWorker(cfg.Kafka.Brokers, cfg.Saga.Topics, cfg.Saga.GroupID, orch, logger)
	defer func() { _ = worker.Close() }()

	logger.Info("saga orchestrator connecting",
		slog.Any("brokers", cfg.Kafka.Brokers),
		slog.String("inventory", cfg.Saga.InventoryBaseURL),
		slog.String("payment_outcome", cfg.Saga.PaymentOutcome),
	)
	return worker.Run(ctx)
}
