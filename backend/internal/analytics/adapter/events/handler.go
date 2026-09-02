package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	analyticsapp "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
)

type Handler struct{ service *analyticsapp.Service }

func NewHandler(service *analyticsapp.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Handle(ctx context.Context, record kafkaadapter.Record) error {
	var event orderevent.Event
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return fmt.Errorf("decode order event: %w", err)
	}
	if err := validate(event, record); err != nil {
		return err
	}
	return h.service.ApplyOrderEvent(ctx, event, analyticsapp.EventMetadata{
		Topic: record.Topic, Partition: record.Partition, Offset: record.Offset,
	})
}

func validate(event orderevent.Event, record kafkaadapter.Record) error {
	if event.SchemaVersion != orderevent.SchemaVersion || event.AggregateType != "order" {
		return fmt.Errorf("unsupported order event contract")
	}
	if _, err := uuid.Parse(event.EventID); err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}
	if _, err := uuid.Parse(event.AggregateID); err != nil {
		return fmt.Errorf("invalid aggregate ID: %w", err)
	}
	if _, err := uuid.Parse(event.Order.UserID); err != nil {
		return fmt.Errorf("invalid order user ID: %w", err)
	}
	if !knownEventType(event.EventType) {
		return fmt.Errorf("unsupported order event type %q", event.EventType)
	}
	if string(record.Key) != event.AggregateID || event.Order.ID != event.AggregateID {
		return fmt.Errorf("kafka key, aggregate ID, and order ID must match")
	}
	if event.EventID != record.Headers["event_id"] || event.EventType != record.Headers["event_type"] {
		return fmt.Errorf("kafka event headers do not match payload")
	}
	if event.OccurredAt.IsZero() || event.Order.Status == "" || len(event.Order.Currency) != 3 || event.Order.TotalCents < 0 {
		return fmt.Errorf("incomplete order event")
	}
	return nil
}

func knownEventType(eventType string) bool {
	switch eventType {
	case orderevent.EventCreated,
		orderevent.EventStockReserved,
		orderevent.EventPaymentPending,
		orderevent.EventConfirmed,
		orderevent.EventCancelled,
		orderevent.EventCompensationPending:
		return true
	default:
		return false
	}
}
