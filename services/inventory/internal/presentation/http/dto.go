package http

// reserveRequest is the POST /v1/reservations body.
type reserveRequest struct {
	OrderID string           `json:"orderId"`
	Lines   []reserveLineDTO `json:"lines"`
}

type reserveLineDTO struct {
	SKU      string `json:"sku"`
	Quantity int32  `json:"quantity"`
}

// reservationResponse is the reservation representation returned to clients.
type reservationResponse struct {
	ReservationID string           `json:"reservationId"`
	OrderID       string           `json:"orderId"`
	Status        string           `json:"status"`
	Lines         []reserveLineDTO `json:"lines"`
}

// stockResponse is the ledger view for one SKU.
type stockResponse struct {
	SKU       string `json:"sku"`
	Available int64  `json:"available"`
	Reserved  int64  `json:"reserved"`
	Version   int64  `json:"version"`
}

// setStockRequest is the PUT /v1/stock/{sku} body.
type setStockRequest struct {
	Available int64 `json:"available"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlationId,omitempty"`
}
