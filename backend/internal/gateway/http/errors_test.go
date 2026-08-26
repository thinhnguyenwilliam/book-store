package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestContextWithTimeoutPropagatesRequestIDToGRPC(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/books", nil)
	recorder := httptest.NewRecorder()
	c := e.NewContext(request, recorder)
	c.Response().Header().Set(echo.HeaderXRequestID, "request-123")
	traceID := "0123456789abcdef0123456789abcdef"
	c.SetRequest(c.Request().WithContext(apptrace.ContextWithID(c.Request().Context(), traceID)))

	ctx := grpcContext(c)
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

func TestProviderErrorResponseReturnsStableMachineReadableCode(t *testing.T) {
	e := echo.New()
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(
		httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/facebook", nil),
		recorder,
	)

	if err := providerErrorResponse(ctx, providerFacebook, status.Error(codes.Unavailable, "raw upstream detail")); err != nil {
		t.Fatalf("providerErrorResponse() error = %v", err)
	}
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusServiceUnavailable ||
		response.Error.Code != "provider_unavailable" ||
		response.Error.Provider != providerFacebook ||
		!response.Error.Retryable {
		t.Fatalf("unexpected provider error response: status=%d body=%+v", recorder.Code, response)
	}
	if response.Error.Message == "raw upstream detail" {
		t.Fatal("raw provider detail must not be exposed")
	}
}

func TestErrorResponseMapsGRPCStatusToHTTP(t *testing.T) {
	tests := []struct {
		name        string
		grpcCode    codes.Code
		wantStatus  int
		wantMessage string
	}{
		{name: "invalid argument", grpcCode: codes.InvalidArgument, wantStatus: http.StatusBadRequest, wantMessage: "domain message"},
		{name: "unauthenticated", grpcCode: codes.Unauthenticated, wantStatus: http.StatusUnauthorized, wantMessage: "domain message"},
		{name: "permission denied", grpcCode: codes.PermissionDenied, wantStatus: http.StatusForbidden, wantMessage: "domain message"},
		{name: "not found", grpcCode: codes.NotFound, wantStatus: http.StatusNotFound, wantMessage: "domain message"},
		{name: "already exists", grpcCode: codes.AlreadyExists, wantStatus: http.StatusConflict, wantMessage: "domain message"},
		{name: "aborted", grpcCode: codes.Aborted, wantStatus: http.StatusConflict, wantMessage: "domain message"},
		{name: "failed precondition", grpcCode: codes.FailedPrecondition, wantStatus: http.StatusPreconditionFailed, wantMessage: "domain message"},
		{name: "resource exhausted", grpcCode: codes.ResourceExhausted, wantStatus: http.StatusTooManyRequests, wantMessage: "domain message"},
		{name: "canceled", grpcCode: codes.Canceled, wantStatus: statusClientClosedRequest, wantMessage: "request canceled"},
		{name: "deadline", grpcCode: codes.DeadlineExceeded, wantStatus: http.StatusGatewayTimeout, wantMessage: "upstream service timed out"},
		{name: "unimplemented", grpcCode: codes.Unimplemented, wantStatus: http.StatusNotImplemented, wantMessage: "operation is not supported"},
		{name: "unavailable", grpcCode: codes.Unavailable, wantStatus: http.StatusServiceUnavailable, wantMessage: "upstream service unavailable"},
		{name: "internal", grpcCode: codes.Internal, wantStatus: http.StatusInternalServerError, wantMessage: "internal server error"},
		{name: "data loss", grpcCode: codes.DataLoss, wantStatus: http.StatusInternalServerError, wantMessage: "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil), recorder)

			if err := errorResponse(ctx, status.Error(tt.grpcCode, "domain message")); err != nil {
				t.Fatalf("errorResponse() error = %v", err)
			}
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			var response ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", response.Error.Message, tt.wantMessage)
			}
		})
	}
}
