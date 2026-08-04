package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/orderfulfillment/order/internal/app/command"
	"github.com/orderfulfillment/order/internal/app/saga"
	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/inventory"
	"github.com/orderfulfillment/order/internal/infra/logging"
	"github.com/orderfulfillment/order/internal/infra/metrics"
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
	err := c.confirm.Handle(ctx, orderID)
	if err == nil {
		metrics.SagaOutcomes.WithLabelValues("confirmed").Inc()
	}
	return err
}

func (c orderController) Cancel(ctx context.Context, orderID, reason string) error {
	err := c.cancel.Handle(ctx, orderID, reason)
	if err == nil {
		metrics.SagaOutcomes.WithLabelValues("cancelled").Inc()
	}
	return err
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

	metricsSrv := metrics.StartServer(cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

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
	var paymentGateway saga.PaymentGateway
	if cfg.Saga.PaymentBaseURL != "" {
		paymentGateway = payment.NewClient(cfg.Saga.PaymentBaseURL, cfg.Saga.HTTPTimeout)
		logger.Info("payment gateway: service", slog.String("url", cfg.Saga.PaymentBaseURL))
	} else {
		paymentGateway = payment.NewStubGateway(cfg.Saga.PaymentOutcome)
		logger.Info("payment gateway: stub", slog.String("outcome", cfg.Saga.PaymentOutcome))
	}
	orch := saga.NewOrchestrator(
		inventory.NewClient(cfg.Saga.InventoryBaseURL, cfg.Saga.HTTPTimeout),
		paymentGateway,
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
