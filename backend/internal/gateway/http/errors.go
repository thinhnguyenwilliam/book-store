package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func contextWithTimeout(c echo.Context) (context.Context, context.CancelFunc) {
	ctx := c.Request().Context()
	if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
	}
	if traceID := apptrace.IDFromContext(ctx); traceID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, apptrace.MetadataKey, traceID)
	}
	return context.WithTimeout(ctx, requestTimeout)
}

func errorResponse(c echo.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return c.JSON(http.StatusGatewayTimeout, errorBody("upstream service timed out"))
	}
	var echoError *echo.HTTPError
	if errors.As(err, &echoError) {
		statusCode := echoError.Code
		if statusCode < http.StatusBadRequest || statusCode >= http.StatusInternalServerError {
			statusCode = http.StatusBadRequest
		}
		return c.JSON(statusCode, errorBody("invalid request"))
	}

	statusCode := http.StatusInternalServerError
	message := "internal server error"
	grpcStatus, ok := status.FromError(err)
	if ok {
		switch grpcStatus.Code() {
		case codes.InvalidArgument:
			statusCode = http.StatusBadRequest
		case codes.Unauthenticated:
			statusCode = http.StatusUnauthorized
		case codes.PermissionDenied:
			statusCode = http.StatusForbidden
		case codes.NotFound:
			statusCode = http.StatusNotFound
		case codes.AlreadyExists:
			statusCode = http.StatusConflict
		case codes.DeadlineExceeded:
			statusCode = http.StatusGatewayTimeout
		case codes.Unavailable:
			statusCode = http.StatusServiceUnavailable
		}
		if statusCode < http.StatusInternalServerError {
			message = grpcStatus.Message()
		}
	}
	return c.JSON(statusCode, errorBody(message))
}

func errorBody(message string) ErrorResponse {
	return ErrorResponse{Error: ErrorDetail{Message: message}}
}
