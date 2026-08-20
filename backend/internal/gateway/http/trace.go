package http

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

func TraceID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		traceID := apptrace.Normalize(c.Request().Header.Get(apptrace.Header))
		if traceID == "" {
			var err error
			traceID, err = apptrace.NewID()
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "could not create trace ID").SetInternal(
					fmt.Errorf("generate trace ID: %w", err),
				)
			}
		}

		c.Response().Header().Set(apptrace.Header, traceID)
		request := c.Request().WithContext(apptrace.ContextWithID(c.Request().Context(), traceID))
		c.SetRequest(request)
		return next(c)
	}
}
