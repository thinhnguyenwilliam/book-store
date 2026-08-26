package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/application"
)

type PaymentHandler struct {
	service *application.Service
}

type paymentEvent struct {
	OrderID string `json:"order_id"`
	BuyerID string `json:"buyer_id"`
	Status  string `json:"status"`
}

func NewPaymentHandler(service *application.Service) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) Handle(ctx context.Context, eventType string, payload []byte) error {
	if eventType != "payment.succeeded" && eventType != "payment.failed" && eventType != "payment.refunded" {
		return nil
	}
	var event paymentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("decode payment event: %w", err)
	}
	if "payment."+event.Status != eventType {
		return fmt.Errorf("payment event type does not match payload status")
	}
	return h.service.HandlePaymentEvent(ctx, event.OrderID, event.BuyerID)
}
