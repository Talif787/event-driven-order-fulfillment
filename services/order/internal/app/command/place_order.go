package command

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/contracts"
	"github.com/orderfulfillment/order/internal/domain/order"
)

// PlaceOrderLine is a transport-agnostic input line.
type PlaceOrderLine struct {
	SKU              string
	Quantity         int32
	UnitPriceMinor   int64
	Currency         string
}

// PlaceOrderCommand is the application input for placing an order.
type PlaceOrderCommand struct {
	CustomerID     string
	IdempotencyKey string
	Lines          []PlaceOrderLine
	Ship           ShipTo
}

// ShipTo mirrors the shipping address at the application boundary.
type ShipTo struct {
	Line1, Line2, City, Region, PostalCode, Country string
}

// PlaceOrderResult reports the outcome of a placement.
type PlaceOrderResult struct {
	OrderID     string
	Status      string
	Idempotent  bool
}

// PlaceOrderHandler orchestrates the place-order use case.
type PlaceOrderHandler struct {
	repo   app.Repository
	idem   app.IdempotencyStore
	ids    app.IDGenerator
	clock  app.Clock
	logger *slog.Logger
	tracer trace.Tracer
}

func NewPlaceOrderHandler(repo app.Repository, idem app.IdempotencyStore, ids app.IDGenerator, clock app.Clock, logger *slog.Logger, tracer trace.Tracer) *PlaceOrderHandler {
	return &PlaceOrderHandler{repo: repo, idem: idem, ids: ids, clock: clock, logger: logger, tracer: tracer}
}

// Handle validates input, enforces idempotency, creates the aggregate, and
// persists its events together with an outbox message in one transaction.
func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrderCommand) (PlaceOrderResult, error) {
	ctx, span := h.tracer.Start(ctx, "PlaceOrderHandler.Handle")
	defer span.End()

	key, err := order.NewIdempotencyKey(cmd.IdempotencyKey)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	customerID, err := order.ParseCustomerID(cmd.CustomerID)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	items, err := buildLineItems(cmd.Lines)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	shipTo, err := order.NewAddress(cmd.Ship.Line1, cmd.Ship.Line2, cmd.Ship.City, cmd.Ship.Region, cmd.Ship.PostalCode, cmd.Ship.Country)
	if err != nil {
		return PlaceOrderResult{}, err
	}

	orderID := h.ids.NewOrderID()

	existing, found, err := h.idem.Reserve(ctx, key.String(), orderID.String())
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("reserve idempotency key: %w", err)
	}
	if found {
		h.logger.InfoContext(ctx, "idempotent replay", slog.String("order_id", existing))
		return PlaceOrderResult{OrderID: existing, Status: string(order.StatusPending), Idempotent: true}, nil
	}

	agg, err := order.PlaceOrder(orderID, customerID, items, shipTo, h.clock.Now())
	if err != nil {
		return PlaceOrderResult{}, err
	}

	outbox, err := toOutbox(agg)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if err := h.repo.Save(ctx, agg, 0, outbox); err != nil {
		return PlaceOrderResult{}, fmt.Errorf("persist order: %w", err)
	}
	agg.MarkChangesCommitted()

	h.logger.InfoContext(ctx, "order placed",
		slog.String("order_id", agg.ID().String()),
		slog.Int64("total_minor", agg.Total().MinorUnits),
		slog.String("currency", agg.Total().Currency),
	)
	return PlaceOrderResult{OrderID: agg.ID().String(), Status: string(agg.Status())}, nil
}

func buildLineItems(lines []PlaceOrderLine) ([]order.LineItem, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: at least one line is required", order.ErrValidation)
	}
	items := make([]order.LineItem, 0, len(lines))
	for _, l := range lines {
		sku, err := order.NewSKU(l.SKU)
		if err != nil {
			return nil, err
		}
		qty, err := order.NewQuantity(l.Quantity)
		if err != nil {
			return nil, err
		}
		price, err := order.NewMoney(l.Currency, l.UnitPriceMinor)
		if err != nil {
			return nil, err
		}
		items = append(items, order.LineItem{SKU: sku, Quantity: qty, UnitPrice: price})
	}
	return items, nil
}

func toOutbox(agg *order.Order) ([]app.OutboxMessage, error) {
	messages := make([]app.OutboxMessage, 0, len(agg.UncommittedChanges()))
	for _, ev := range agg.UncommittedChanges() {
		placed, ok := ev.(order.OrderPlaced)
		if !ok {
			continue
		}
		items := make([]contracts.LineItem, 0, len(placed.Items))
		for _, it := range placed.Items {
			items = append(items, contracts.LineItem{
				SKU: it.SKU.String(), Quantity: it.Quantity.Value(), UnitPriceMinor: it.UnitPrice.MinorUnits,
			})
		}
		event := contracts.OrderPlacedV1{
			OrderID:    placed.OrderID.String(),
			CustomerID: placed.CustomerID.String(),
			Items:      items,
			ShipTo: contracts.Address{
				Line1: placed.ShipTo.Line1, Line2: placed.ShipTo.Line2, City: placed.ShipTo.City,
				Region: placed.ShipTo.Region, PostalCode: placed.ShipTo.PostalCode, Country: placed.ShipTo.Country,
			},
			TotalMinor: placed.Total.MinorUnits,
			Currency:   placed.Total.Currency,
			PlacedAt:   contracts.FormatTime(placed.PlacedAt),
		}
		raw, err := event.Marshal()
		if err != nil {
			return nil, err
		}
		messages = append(messages, app.OutboxMessage{
			AggregateID: placed.OrderID.String(),
			Topic:       contracts.TopicOrderEvents,
			EventType:   string(ev.EventType()),
			Payload:     raw,
			Headers:     map[string]string{contracts.HeaderContentType: "application/json"},
		})
	}
	return messages, nil
}
