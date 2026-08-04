package http

import "github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"

// dispatchRequest is the POST /v1/shipments/{orderId}/dispatch body.
type dispatchRequest struct {
	Carrier      string `json:"carrier"`
	TrackingCode string `json:"trackingCode"`
}

// shipmentResponse is the shipment representation returned to clients.
type shipmentResponse struct {
	ShipmentID    string `json:"shipmentId"`
	OrderID       string `json:"orderId"`
	Status        string `json:"status"`
	Carrier       string `json:"carrier"`
	TrackingCode  string `json:"trackingCode"`
	FailureReason string `json:"failureReason"`
	Version       int64  `json:"version"`
}

func toShipmentResponse(s *fulfillment.Shipment) shipmentResponse {
	return shipmentResponse{
		ShipmentID:    s.ID().String(),
		OrderID:       s.OrderID().String(),
		Status:        string(s.Status()),
		Carrier:       s.Carrier(),
		TrackingCode:  s.TrackingCode(),
		FailureReason: s.FailureReason(),
		Version:       s.Version(),
	}
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlationId"`
}
