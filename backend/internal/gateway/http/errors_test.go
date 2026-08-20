package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc/metadata"
)

func TestContextWithTimeoutPropagatesRequestIDToGRPC(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/books", nil)
	recorder := httptest.NewRecorder()
	c := e.NewContext(request, recorder)
	c.Response().Header().Set(echo.HeaderXRequestID, "request-123")
	traceID := "0123456789abcdef0123456789abcdef"
	c.SetRequest(c.Request().WithContext(apptrace.ContextWithID(c.Request().Context(), traceID)))

	ctx, cancel := contextWithTimeout(c)
	defer cancel()
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing gRPC metadata is missing")
	}
	if got := md.Get("x-request-id"); len(got) != 1 || got[0] != "request-123" {
		t.Fatalf("x-request-id metadata = %v, want [request-123]", got)
	}
	if got := md.Get(apptrace.MetadataKey); len(got) != 1 || got[0] != traceID {
		t.Fatalf("trace ID metadata = %v, want [%s]", got, traceID)
	}
}
