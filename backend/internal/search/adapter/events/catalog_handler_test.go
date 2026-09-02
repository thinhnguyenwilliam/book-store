package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	catalogevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/catalog"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
	searchapp "github.com/thinhnguyenwilliam/book-store/backend/internal/search/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

func TestCatalogHandlerValidatesContractAndUpserts(t *testing.T) {
	index := &handlerIndexStub{}
	handler := NewCatalogHandler(searchapp.NewService(index))
	now := time.Now().UTC()
	event := catalogevent.Event{
		EventID: uuid.NewString(), EventType: catalogevent.EventBookUpserted,
		SchemaVersion: catalogevent.SchemaVersion, BookID: uuid.NewString(), Version: now.UnixNano(),
		OccurredAt: now,
	}
	event.Book = &catalogevent.Book{ID: event.BookID, Title: "Clean Architecture", UpdatedAt: now}
	payload, _ := json.Marshal(event)
	err := handler.Handle(context.Background(), kafkaadapter.Record{
		Key: []byte(event.BookID), Value: payload,
		Headers: map[string]string{"event_id": event.EventID, "event_type": event.EventType},
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if index.upserted.ID != event.BookID || index.version != event.Version {
		t.Fatalf("unexpected upsert: %+v version=%d", index.upserted, index.version)
	}

	badRecord := kafkaadapter.Record{Key: []byte(uuid.NewString()), Value: payload, Headers: map[string]string{
		"event_id": event.EventID, "event_type": event.EventType,
	}}
	if err := handler.Handle(context.Background(), badRecord); err == nil {
		t.Fatal("Handle() error = nil for mismatched Kafka key")
	}
}

type handlerIndexStub struct {
	upserted domain.BookDocument
	version  int64
}

func (s *handlerIndexStub) Ensure(context.Context) (bool, error) { return false, nil }
func (s *handlerIndexStub) Upsert(_ context.Context, book domain.BookDocument, version int64) error {
	s.upserted, s.version = book, version
	return nil
}
func (s *handlerIndexStub) Delete(context.Context, string, int64) error             { return nil }
func (s *handlerIndexStub) BulkUpsert(context.Context, []domain.BookDocument) error { return nil }
func (s *handlerIndexStub) Search(context.Context, domain.Request) (domain.Result, error) {
	return domain.Result{}, nil
}
func (s *handlerIndexStub) Suggest(context.Context, string, int) (domain.Result, error) {
	return domain.Result{}, nil
}
