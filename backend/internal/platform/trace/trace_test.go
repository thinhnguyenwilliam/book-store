package trace

import (
	"context"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestTraceIDRoundTrip(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if len(id) != IDLength || Normalize(id) != id {
		t.Fatalf("NewID() = %q, want %d lowercase hex characters", id, IDLength)
	}
	if got := IDFromContext(ContextWithID(context.Background(), id)); got != id {
		t.Fatalf("IDFromContext() = %q, want %q", got, id)
	}
}

func TestEnsureSpanContextUsesApplicationTraceID(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := EnsureSpanContext(ContextWithID(context.Background(), traceID))
	spanContext := oteltrace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() || spanContext.TraceID().String() != traceID {
		t.Fatalf("span context trace ID = %q, want %q", spanContext.TraceID().String(), traceID)
	}
}

func TestNormalizeRejectsInvalidTraceID(t *testing.T) {
	for _, id := range []string{"", "short", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		if got := Normalize(id); got != "" {
			t.Fatalf("Normalize(%q) = %q, want empty", id, got)
		}
	}
}
