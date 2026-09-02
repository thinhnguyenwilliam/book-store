package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	catalogevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/catalog"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
	searchapp "github.com/thinhnguyenwilliam/book-store/backend/internal/search/application"
)

type CatalogHandler struct{ service *searchapp.Service }

func NewCatalogHandler(service *searchapp.Service) *CatalogHandler {
	return &CatalogHandler{service: service}
}

func (h *CatalogHandler) Handle(ctx context.Context, record kafkaadapter.Record) error {
	var event catalogevent.Event
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return fmt.Errorf("decode catalog event: %w", err)
	}
	if event.SchemaVersion != catalogevent.SchemaVersion || !catalogevent.KnownEventType(event.EventType) {
		return fmt.Errorf("unsupported catalog event contract")
	}
	if _, err := uuid.Parse(event.EventID); err != nil {
		return fmt.Errorf("invalid catalog event ID: %w", err)
	}
	if _, err := uuid.Parse(event.BookID); err != nil {
		return fmt.Errorf("invalid catalog book ID: %w", err)
	}
	if event.Version < 1 || event.OccurredAt.IsZero() {
		return fmt.Errorf("incomplete catalog event")
	}
	if string(record.Key) != event.BookID || record.Headers["event_id"] != event.EventID || record.Headers["event_type"] != event.EventType {
		return fmt.Errorf("kafka key or event headers do not match catalog payload")
	}
	if event.EventType == catalogevent.EventBookUpserted && (event.Book == nil || event.Book.ID != event.BookID) {
		return fmt.Errorf("catalog upsert snapshot does not match aggregate")
	}
	return h.service.ApplyCatalogEvent(ctx, event)
}
