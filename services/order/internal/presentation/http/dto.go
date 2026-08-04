package http

// placeOrderRequest is the REST payload for creating an order. The customer id
// is taken from the authenticated principal when auth is enabled; the body
// field is used only in unauthenticated local development.
type placeOrderRequest struct {
	CustomerID string     `json:"customerId,omitempty"`
	Items      []itemDTO  `json:"items"`
	ShipTo     addressDTO `json:"shipTo"`
}

type itemDTO struct {
	SKU            string `json:"sku"`
	Quantity       int32  `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Currency       string `json:"currency"`
}

type addressDTO struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

type placeOrderResponse struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
}

type orderResponse struct {
	OrderID    string `json:"orderId"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
	TotalMinor int64  `json:"totalMinor"`
	Currency   string `json:"currency"`
	Version    int64  `json:"version"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlationId,omitempty"`
}
