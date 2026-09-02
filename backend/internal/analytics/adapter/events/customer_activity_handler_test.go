package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	analyticsapp "github.com/thinhnguyenwilliam/book-store/backend/internal/analytics/application"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	kafkaadapter "github.com/thinhnguyenwilliam/book-store/backend/internal/messaging/kafka"
)

func TestCustomerActivityHandlerValidEvent(t *testing.T) {
	repository := &activityRepositoryStub{repositoryStub: repositoryStub{}}
	handler := NewCustomerActivityHandler(analyticsapp.NewService(repository))
	event := customeractivity.Event{
		EventID: uuid.NewString(), EventType: customeractivity.EventBookViewed,
		SchemaVersion: customeractivity.SchemaVersion, ActorID: uuid.NewString(),
		AnonymousID: uuid.NewString(), SessionID: uuid.NewString(), BookID: uuid.NewString(),
		Source: "storefront", OccurredAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	record := kafkaadapter.Record{
		Topic: "bookstore.customer-activity", Partition: 3, Offset: 12,
		Key: []byte(event.ActorID), Value: payload,
		Headers: map[string]string{"event_id": event.EventID, "event_type": event.EventType},
	}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.activity.EventID != event.EventID || repository.activityMetadata.Offset != 12 {
		t.Fatalf("activity not applied: %+v", repository.activity)
	}
}

func TestCustomerActivityHandlerRejectsSpoofedHeader(t *testing.T) {
	repository := &activityRepositoryStub{repositoryStub: repositoryStub{}}
	handler := NewCustomerActivityHandler(analyticsapp.NewService(repository))
	event := customeractivity.Event{
		EventID: uuid.NewString(), EventType: customeractivity.EventCheckoutStarted,
		SchemaVersion: customeractivity.SchemaVersion, ActorID: uuid.NewString(),
		Source: "storefront", OccurredAt: time.Now().UTC(),
	}
	payload, _ := json.Marshal(event)
	err := handler.Handle(context.Background(), kafkaadapter.Record{
		Key: []byte(event.ActorID), Value: payload,
		Headers: map[string]string{"event_id": uuid.NewString(), "event_type": event.EventType},
	})
	if err == nil {
		t.Fatal("Handle() error = nil, want header validation error")
	}
}

type activityRepositoryStub struct {
	repositoryStub
	activity         customeractivity.Event
	activityMetadata analyticsapp.EventMetadata
}

func (r *activityRepositoryStub) ApplyCustomerActivity(_ context.Context, event customeractivity.Event, metadata analyticsapp.EventMetadata) error {
	r.activity, r.activityMetadata = event, metadata
	return nil
}
