package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	Header      = "X-Trace-ID"
	MetadataKey = "x-trace-id"
	IDLength    = 32
)

type contextKey struct{}

func NewID() (string, error) {
	bytes := make([]byte, IDLength/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func Normalize(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if len(id) != IDLength {
		return ""
	}
	if _, err := hex.DecodeString(id); err != nil {
		return ""
	}
	return id
}

func ContextWithID(ctx context.Context, id string) context.Context {
	if normalized := Normalize(id); normalized != "" {
		return context.WithValue(ctx, contextKey{}, normalized)
	}
	return ctx
}

// ContextWithRemoteID seeds a valid W3C parent when callers only provide the
// legacy X-Trace-ID header. The first server span will continue this trace ID.
func ContextWithRemoteID(ctx context.Context, id string) (context.Context, error) {
	traceID, err := oteltrace.TraceIDFromHex(Normalize(id))
	if err != nil {
		return ctx, err
	}
	var spanID oteltrace.SpanID
	if _, err := rand.Read(spanID[:]); err != nil {
		return ctx, err
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	return oteltrace.ContextWithRemoteSpanContext(ctx, spanContext), nil
}

// EnsureSpanContext bridges the application's legacy trace ID into an
// OpenTelemetry parent. It is mainly used for asynchronous messages created
// from an outbox, where only the stable trace ID may remain in the context.
func EnsureSpanContext(ctx context.Context) context.Context {
	if oteltrace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	seeded, err := ContextWithRemoteID(ctx, IDFromContext(ctx))
	if err != nil {
		return ctx
	}
	return seeded
}

// SyncID stores the active OpenTelemetry trace ID for the slog handler and
// existing X-Trace-ID integrations.
func SyncID(ctx context.Context) context.Context {
	spanContext := oteltrace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ctx
	}
	return ContextWithID(ctx, spanContext.TraceID().String())
}

func IDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
