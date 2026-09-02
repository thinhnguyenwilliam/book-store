package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	analyticsapp "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
)

type CustomerActivityHandler struct{ service *analyticsapp.Service }

func NewCustomerActivityHandler(service *analyticsapp.Service) *CustomerActivityHandler {
	return &CustomerActivityHandler{service: service}
}

func (h *CustomerActivityHandler) Handle(ctx context.Context, record kafkaadapter.Record) error {
	var event customeractivity.Event
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return fmt.Errorf("decode customer activity: %w", err)
	}
	if err := validateCustomerActivity(event, record); err != nil {
		return err
	}
	return h.service.ApplyCustomerActivity(ctx, event, analyticsapp.EventMetadata{
		Topic: record.Topic, Partition: record.Partition, Offset: record.Offset,
	})
}

func validateCustomerActivity(event customeractivity.Event, record kafkaadapter.Record) error {
	if event.SchemaVersion != customeractivity.SchemaVersion || !customeractivity.KnownEventType(event.EventType) {
		return fmt.Errorf("unsupported customer activity contract")
	}
	for name, value := range map[string]string{"event ID": event.EventID, "actor ID": event.ActorID} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("invalid %s: %w", name, err)
		}
	}
	for name, value := range map[string]string{
		"user ID": event.UserID, "anonymous ID": event.AnonymousID, "session ID": event.SessionID,
		"book ID": event.BookID, "order ID": event.OrderID, "comment ID": event.CommentID,
	} {
		if value != "" {
			if _, err := uuid.Parse(value); err != nil {
				return fmt.Errorf("invalid %s: %w", name, err)
			}
		}
	}
	if string(record.Key) != event.ActorID || event.EventID != record.Headers["event_id"] || event.EventType != record.Headers["event_type"] {
		return fmt.Errorf("kafka key or event headers do not match customer activity payload")
	}
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.Source) == "" || len(event.Query) > 200 || event.Quantity < 0 || event.Quantity > 100 {
		return fmt.Errorf("incomplete customer activity event")
	}
	if event.EventType == customeractivity.EventBookSearched && len(strings.TrimSpace(event.Query)) < 2 {
		return fmt.Errorf("book.searched requires a query")
	}
	if requiresBook(event.EventType) && event.BookID == "" {
		return fmt.Errorf("%s requires a book ID", event.EventType)
	}
	if (event.EventType == customeractivity.EventBookAddedToCart || event.EventType == customeractivity.EventBookRemovedFromCart) && event.Quantity < 1 {
		return fmt.Errorf("%s requires a positive quantity", event.EventType)
	}
	if event.EventType == customeractivity.EventOrderConfirmed && event.OrderID == "" {
		return fmt.Errorf("order.confirmed requires an order ID")
	}
	if event.EventType == customeractivity.EventCommentCreated && event.CommentID == "" {
		return fmt.Errorf("comment.created requires a comment ID")
	}
	return nil
}

func requiresBook(eventType string) bool {
	switch eventType {
	case customeractivity.EventBookViewed, customeractivity.EventBookAddedToCart,
		customeractivity.EventBookRemovedFromCart, customeractivity.EventCommentCreated:
		return true
	default:
		return false
	}
}
