package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

func TestTraceIDPreservesValidIncomingID(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(apptrace.Header, traceID)
	recorder := httptest.NewRecorder()
	c := e.NewContext(request, recorder)

	err := TraceID(func(c echo.Context) error {
		if got := apptrace.IDFromContext(c.Request().Context()); got != traceID {
			t.Fatalf("context trace ID = %q, want %q", got, traceID)
		}
		return c.NoContent(http.StatusNoContent)
	})(c)
	if err != nil {
		t.Fatalf("TraceID() error = %v", err)
	}
	if got := recorder.Header().Get(apptrace.Header); got != traceID {
		t.Fatalf("response trace ID = %q, want %q", got, traceID)
	}
}

func TestTraceIDReplacesInvalidIncomingID(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(apptrace.Header, "not-valid")
	recorder := httptest.NewRecorder()
	c := e.NewContext(request, recorder)

	err := TraceID(func(c echo.Context) error {
		if got := apptrace.IDFromContext(c.Request().Context()); apptrace.Normalize(got) == "" {
			t.Fatalf("generated trace ID is invalid: %q", got)
		}
		return c.NoContent(http.StatusNoContent)
	})(c)
	if err != nil {
		t.Fatalf("TraceID() error = %v", err)
	}
}
