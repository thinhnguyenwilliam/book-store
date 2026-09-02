package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

type Publisher interface {
	Publish(context.Context, string, string, string, []byte) error
}

type Tracker struct {
	publisher Publisher
	queue     chan customeractivity.Event
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewTracker(publisher Publisher, bufferSize int) (*Tracker, error) {
	if publisher == nil || bufferSize < 1 {
		return nil, fmt.Errorf("activity publisher and positive buffer size are required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tracker{publisher: publisher, queue: make(chan customeractivity.Event, bufferSize), ctx: ctx, cancel: cancel}
	t.wg.Add(1)
	go t.run()
	return t, nil
}

// Record is deliberately non-blocking. Customer analytics must never increase
// checkout latency or make a domain operation fail when Kafka is unavailable.
func (t *Tracker) Record(event customeractivity.Event) bool {
	select {
	case t.queue <- event:
		return true
	default:
		slog.Warn("customer activity buffer full; event dropped", "event_id", event.EventID, "event_type", event.EventType)
		return false
	}
}

func (t *Tracker) Close(ctx context.Context) error {
	t.cancel()
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *Tracker) run() {
	defer t.wg.Done()
	for {
		select {
		case event := <-t.queue:
			t.publish(event)
		case <-t.ctx.Done():
			for {
				select {
				case event := <-t.queue:
					t.publish(event)
				default:
					return
				}
			}
		}
	}
}

func (t *Tracker) publish(event customeractivity.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("marshal customer activity", "event_id", event.EventID, "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = apptrace.ContextWithID(ctx, event.TraceID)
	if err := t.publisher.Publish(ctx, event.EventID, event.EventType, event.ActorID, payload); err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("publish customer activity failed", "event_id", event.EventID, "event_type", event.EventType, "error", err)
	}
}
