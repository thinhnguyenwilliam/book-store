package kafka

import (
	"context"
	"testing"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestRecordPrefersW3CTraceContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	const traceID = "11111111111111111111111111111111"
	record := Record{Headers: map[string]string{
		"trace_id":    "22222222222222222222222222222222",
		"traceparent": "00-" + traceID + "-3333333333333333-01",
	}}
	ctx := contextWithRecordTrace(context.Background(), record)
	if got := apptrace.IDFromContext(ctx); got != traceID {
		t.Fatalf("handler trace ID = %q, want W3C trace ID %q", got, traceID)
	}
}

func TestRecordFallsBackToLegacyTraceID(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := contextWithRecordTrace(context.Background(), Record{Headers: map[string]string{"trace_id": traceID}})
	if got := apptrace.IDFromContext(ctx); got != traceID {
		t.Fatalf("handler trace ID = %q, want %q", got, traceID)
	}
}
