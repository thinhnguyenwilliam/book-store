package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestRequestDeadlineAddsRequestContextDeadline(t *testing.T) {
	e := echo.New()
	e.Use(RequestDeadline(200 * time.Millisecond))
	e.GET("/", func(c echo.Context) error {
		deadline, ok := c.Request().Context().Deadline()
		if !ok {
			t.Fatal("request context deadline is missing")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 200*time.Millisecond {
			t.Fatalf("remaining deadline = %s", remaining)
		}
		return c.NoContent(http.StatusNoContent)
	})

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
