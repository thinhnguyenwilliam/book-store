package logger

import (
	"context"
	"log/slog"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

type contextHandler struct {
	next slog.Handler
}

func (h contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if traceID := apptrace.IDFromContext(ctx); traceID != "" {
		record.AddAttrs(slog.String("trace_id", traceID))
	}
	return h.next.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return contextHandler{next: h.next.WithAttrs(attributes)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{next: h.next.WithGroup(name)}
}
