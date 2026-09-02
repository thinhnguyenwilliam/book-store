package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	analyticsapp "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	analyticsdomain "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/domain"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
)

type repositoryStub struct {
	applied  orderevent.Event
	metadata analyticsapp.EventMetadata
}

func (r *repositoryStub) ApplyOrderEvent(
	_ context.Context,
	event orderevent.Event,
	metadata analyticsapp.EventMetadata,
) error {
	r.applied, r.metadata = event, metadata
	return nil
}

func (r *repositoryStub) OrderReport(
	_ context.Context,
	_, _ time.Time,
) (analyticsdomain.OrderReport, error) {
	return analyticsdomain.OrderReport{}, nil
}

func (r *repositoryStub) ApplyCustomerActivity(context.Context, customeractivity.Event, analyticsapp.EventMetadata) error {
	return nil
}

func (r *repositoryStub) CustomerActivityReport(context.Context, time.Time, time.Time, int) (analyticsdomain.CustomerActivityReport, error) {
	return analyticsdomain.CustomerActivityReport{}, nil
}

func (r *repositoryStub) TrendingBooks(context.Context, time.Time, time.Time, int) ([]analyticsdomain.BookActivityMetric, error) {
	return nil, nil
}

func (r *repositoryStub) RelatedBooks(context.Context, string, time.Time, time.Time, int) ([]analyticsdomain.RelatedBook, error) {
	return nil, nil
}

func TestHandleValidOrderEvent(t *testing.T) {
	repository := &repositoryStub{}
	handler := NewHandler(analyticsapp.NewService(repository))
	event := testEvent()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	record := kafkaadapter.Record{
		Topic: "bookstore.order-events", Partition: 2, Offset: 42,
		Key: []byte(event.AggregateID), Value: payload,
		Headers: map[string]string{"event_id": event.EventID, "event_type": event.EventType},
	}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.applied.EventID != event.EventID || repository.metadata.Offset != 42 {
		t.Fatalf("event was not mapped to repository: %+v %+v", repository.applied, repository.metadata)
	}
}

func TestHandleRejectsMismatchedPartitionKey(t *testing.T) {
	repository := &repositoryStub{}
	handler := NewHandler(analyticsapp.NewService(repository))
	event := testEvent()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	record := kafkaadapter.Record{
		Topic: "bookstore.order-events", Key: []byte(uuid.NewString()), Value: payload,
		Headers: map[string]string{"event_id": event.EventID, "event_type": event.EventType},
	}
	if err := handler.Handle(context.Background(), record); err == nil {
		t.Fatal("Handle() error = nil, want mismatched key error")
	}
	if repository.applied.EventID != "" {
		t.Fatal("invalid event reached repository")
	}
}

func testEvent() orderevent.Event {
	orderID := uuid.NewString()
	return orderevent.Event{
		EventID: uuid.NewString(), EventType: orderevent.EventCreated,
		SchemaVersion: orderevent.SchemaVersion, AggregateType: "order", AggregateID: orderID,
		OccurredAt: time.Now().UTC(),
		Order: orderevent.Snapshot{
			ID: orderID, UserID: uuid.NewString(), Status: "pending", TotalCents: 100_000, Currency: "VND",
		},
	}
}
