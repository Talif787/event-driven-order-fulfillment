package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/orderfulfillment/fulfillment/internal/contracts"
	"github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"
)

// Service is the fulfillment use-case layer. It creates a shipment when an order
// is confirmed and advances it through dispatch and delivery, publishing an
// event on each real state change.
//
// Events are published after the database write. This is a deliberate
// simplification: a crash between the commit and the publish could drop an
// event. The transactional outbox used by the order service is the production
// hardening; it is deferred here. Downstream consumers dedupe, so a retry that
// republishes is harmless.
type Service struct {
	repo      ShipmentRepository
	publisher EventPublisher
	clock     Clock
	logger    *slog.Logger
}

func NewService(repo ShipmentRepository, publisher EventPublisher, clock Clock, logger *slog.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, clock: clock, logger: logger}
}

// CreateForOrder opens a shipment for a confirmed order. It is idempotent on the
// order id: a redelivered order.confirmed returns the existing shipment. While
// the shipment is still in CREATED it (re)publishes shipment.created, so an
// event lost to a publish failure is recovered on the next delivery without
// creating a duplicate shipment.
func (s *Service) CreateForOrder(ctx context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error) {
	shipment, err := s.repo.GetByOrderID(ctx, orderID)
	switch {
	case err == nil:
		// already exists
	case errors.Is(err, fulfillment.ErrShipmentNotFound):
		shipment, err = fulfillment.Create(orderID, s.clock.Now())
		if err != nil {
			return nil, err
		}
		if err := s.repo.Insert(ctx, shipment); err != nil {
			if errors.Is(err, fulfillment.ErrShipmentExists) {
				// A concurrent delivery won the race; load the winner.
				if shipment, err = s.repo.GetByOrderID(ctx, orderID); err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("insert shipment: %w", err)
			}
		} else {
			s.logger.InfoContext(ctx, "shipment created",
				slog.String("order_id", orderID.String()), slog.String("shipment_id", shipment.ID().String()))
		}
	default:
		return nil, fmt.Errorf("load shipment: %w", err)
	}

	if shipment.Status() == fulfillment.StatusCreated {
		evt := contracts.ShipmentCreatedV1{
			ShipmentID: shipment.ID().String(),
			OrderID:    shipment.OrderID().String(),
			Status:     string(shipment.Status()),
			CreatedAt:  contracts.FormatTime(shipment.CreatedAt()),
		}
		if err := s.publish(ctx, contracts.TypeShipmentCreated, shipment.OrderID(), evt); err != nil {
			return nil, err
		}
	}
	return shipment, nil
}

// Dispatch advances a shipment to DISPATCHED and publishes on a real change.
func (s *Service) Dispatch(ctx context.Context, orderID uuid.UUID, carrier, trackingCode string) (*fulfillment.Shipment, error) {
	shipment, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	changed, err := shipment.Dispatch(carrier, trackingCode, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := s.repo.Update(ctx, shipment); err != nil {
			return nil, err
		}
		evt := contracts.ShipmentDispatchedV1{
			ShipmentID:   shipment.ID().String(),
			OrderID:      shipment.OrderID().String(),
			Carrier:      shipment.Carrier(),
			TrackingCode: shipment.TrackingCode(),
			DispatchedAt: contracts.FormatTime(shipment.UpdatedAt()),
		}
		if err := s.publish(ctx, contracts.TypeShipmentDispatched, orderID, evt); err != nil {
			return nil, err
		}
	}
	return shipment, nil
}

// Deliver advances a shipment to DELIVERED and publishes on a real change.
func (s *Service) Deliver(ctx context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error) {
	shipment, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	changed, err := shipment.Deliver(s.clock.Now())
	if err != nil {
		return nil, err
	}
	if changed {
		if err := s.repo.Update(ctx, shipment); err != nil {
			return nil, err
		}
		evt := contracts.ShipmentDeliveredV1{
			ShipmentID:  shipment.ID().String(),
			OrderID:     shipment.OrderID().String(),
			DeliveredAt: contracts.FormatTime(shipment.UpdatedAt()),
		}
		if err := s.publish(ctx, contracts.TypeShipmentDelivered, orderID, evt); err != nil {
			return nil, err
		}
	}
	return shipment, nil
}

// Get returns the shipment for an order.
func (s *Service) Get(ctx context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error) {
	return s.repo.GetByOrderID(ctx, orderID)
}

type marshaler interface {
	Marshal() ([]byte, error)
}

func (s *Service) publish(ctx context.Context, eventType string, orderID uuid.UUID, evt marshaler) error {
	payload, err := evt.Marshal()
	if err != nil {
		return err
	}
	if err := s.publisher.Publish(ctx, Event{Type: eventType, Key: orderID.String(), Payload: payload}); err != nil {
		return fmt.Errorf("publish %s: %w", eventType, err)
	}
	s.logger.InfoContext(ctx, "published event", slog.String("type", eventType), slog.String("order_id", orderID.String()))
	return nil
}
