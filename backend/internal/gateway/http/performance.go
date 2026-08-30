package http

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestDeadline bounds the total time available to the gateway and all
// downstream gRPC calls. The performance target is observed separately; this
// deadline is deliberately more generous than the latency SLO.
func RequestDeadline(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().URL.Path == "/api/v1/chat/ws" {
				return next(c)
			}
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
