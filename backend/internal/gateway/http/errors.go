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

const statusClientClosedRequest = 499

// grpcContext carries the HTTP request deadline/cancellation and correlation
// metadata into downstream gRPC calls. The gRPC client interceptor applies a
// configurable per-call timeout, so this layer must not hide another timeout.
func grpcContext(c echo.Context) context.Context {
	ctx := c.Request().Context()
	if requestID := c.Response().Header().Get(echo.HeaderXRequestID); requestID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
	}
	if traceID := apptrace.IDFromContext(ctx); traceID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, apptrace.MetadataKey, traceID)
	}
	return ctx
}

func errorResponse(c echo.Context, err error) error {
	if errors.Is(err, context.Canceled) {
		return c.JSON(statusClientClosedRequest, errorBody("request canceled"))
	}
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

	grpcStatus, ok := status.FromError(err)
	if !ok {
		return c.JSON(http.StatusInternalServerError, errorBody("internal server error"))
	}
	statusCode, message := grpcToHTTP(grpcStatus.Code(), grpcStatus.Message())
	return c.JSON(statusCode, errorBody(message))
}

func grpcToHTTP(code codes.Code, grpcMessage string) (int, string) {
	switch code {
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest, grpcMessage
	case codes.Unauthenticated:
		return http.StatusUnauthorized, grpcMessage
	case codes.PermissionDenied:
		return http.StatusForbidden, grpcMessage
	case codes.NotFound:
		return http.StatusNotFound, grpcMessage
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict, grpcMessage
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed, grpcMessage
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, grpcMessage
	case codes.Canceled:
		return statusClientClosedRequest, "request canceled"
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "upstream service timed out"
	case codes.Unimplemented:
		return http.StatusNotImplemented, "operation is not supported"
	case codes.Unavailable:
		return http.StatusServiceUnavailable, "upstream service unavailable"
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func errorBody(message string) ErrorResponse {
	return ErrorResponse{Error: ErrorDetail{Message: message}}
}

func providerErrorResponse(c echo.Context, provider string, err error) error {
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return providerError(c, http.StatusInternalServerError, provider, "external_login_failed", "external login failed", false)
	}

	switch grpcStatus.Code() {
	case codes.Unauthenticated:
		return providerError(c, http.StatusUnauthorized, provider, "invalid_provider_credential", "provider credential is invalid or account is not allowed", false)
	case codes.AlreadyExists:
		return providerError(c, http.StatusConflict, provider, "external_identity_conflict", "provider identity conflicts with an existing account", false)
	case codes.FailedPrecondition:
		return providerError(c, http.StatusPreconditionFailed, provider, "provider_not_configured", "external identity provider is not configured", false)
	case codes.DeadlineExceeded:
		return providerError(c, http.StatusGatewayTimeout, provider, "provider_timeout", "external identity provider timed out", true)
	case codes.Unavailable:
		return providerError(c, http.StatusServiceUnavailable, provider, "provider_unavailable", "external identity provider is unavailable", true)
	default:
		statusCode, _ := grpcToHTTP(grpcStatus.Code(), grpcStatus.Message())
		return providerError(c, statusCode, provider, "external_login_failed", "external login failed", statusCode >= http.StatusInternalServerError)
	}
}

func providerError(c echo.Context, statusCode int, provider, code, message string, retryable bool) error {
	return c.JSON(statusCode, ErrorResponse{Error: ErrorDetail{
		Message:   message,
		Code:      code,
		Provider:  provider,
		Retryable: retryable,
	}})
}
