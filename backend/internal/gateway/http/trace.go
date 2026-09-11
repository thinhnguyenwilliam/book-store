package http

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TraceID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		request := c.Request()
		ctx := otel.GetTextMapPropagator().Extract(
			request.Context(),
			propagation.HeaderCarrier(request.Header),
		)
		spanContext := oteltrace.SpanContextFromContext(ctx)
		traceID := ""
		if spanContext.IsValid() {
			traceID = spanContext.TraceID().String()
		}
		if traceID == "" {
			traceID = apptrace.Normalize(request.Header.Get(apptrace.Header))
		}
		if traceID == "" {
			var err error
			traceID, err = apptrace.NewID()
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "could not create trace ID").SetInternal(
					fmt.Errorf("generate trace ID: %w", err),
				)
			}
		}
		if !spanContext.IsValid() {
			var err error
			ctx, err = apptrace.ContextWithRemoteID(ctx, traceID)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "could not seed trace context").SetInternal(err)
			}
		}

		c.Response().Header().Set(apptrace.Header, traceID)
		request = request.WithContext(apptrace.ContextWithID(ctx, traceID))
		c.SetRequest(request)
		return next(c)
	}
}
