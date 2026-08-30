package rabbitmq

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

func TestDeliveryTraceIDIsAddedToHandlerContext(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	delivery := amqp.Delivery{Headers: amqp.Table{"trace_id": traceID}}
	ctx := contextWithDeliveryTraceID(context.Background(), delivery)
	if got := apptrace.IDFromContext(ctx); got != traceID {
		t.Fatalf("handler trace ID = %q, want %q", got, traceID)
	}
}

func TestDeliveryAttemptUsesQuorumQueueDeliveryCount(t *testing.T) {
	delivery := amqp.Delivery{Headers: amqp.Table{"x-delivery-count": int64(4)}}
	if got := deliveryAttempt(delivery); got != 5 {
		t.Fatalf("deliveryAttempt() = %d, want 5", got)
	}
}

func TestDeliveryEventIDIsAddedToHandlerContext(t *testing.T) {
	delivery := amqp.Delivery{MessageId: "outbox-event-123"}
	ctx := contextWithDeliveryEventID(context.Background(), delivery)
	if got := EventIDFromContext(ctx); got != delivery.MessageId {
		t.Fatalf("handler event ID = %q, want %q", got, delivery.MessageId)
	}
}

func TestRetryDelayIsExponentiallyBounded(t *testing.T) {
	if got := retryDelay(1); got != 500*time.Millisecond {
		t.Fatalf("first retry delay = %s", got)
	}
	if got := retryDelay(100); got != 5*time.Second {
		t.Fatalf("bounded retry delay = %s", got)
	}
}
