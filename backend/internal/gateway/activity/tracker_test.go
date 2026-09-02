package activity

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
)

type publisherStub struct {
	mu      sync.Mutex
	eventID string
	key     string
}

func (p *publisherStub) Publish(_ context.Context, eventID, _, key string, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eventID, p.key = eventID, key
	return nil
}

func TestTrackerPublishesQueuedActivity(t *testing.T) {
	publisher := &publisherStub{}
	tracker, err := NewTracker(publisher, 4)
	if err != nil {
		t.Fatal(err)
	}
	event := customeractivity.Event{EventID: uuid.NewString(), ActorID: uuid.NewString()}
	if !tracker.Record(event) {
		t.Fatal("Record() = false")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tracker.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.eventID != event.EventID || publisher.key != event.ActorID {
		t.Fatalf("unexpected publish: id=%q key=%q", publisher.eventID, publisher.key)
	}
}
