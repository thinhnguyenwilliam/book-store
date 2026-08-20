package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const requestIDMetadataKey = "x-request-id"

func unaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (response any, err error) {
	ctx = contextWithTraceID(ctx)
	startedAt := time.Now()
	requestID := incomingRequestID(ctx)

	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "gRPC unary panic recovered",
				"rpc_method", info.FullMethod,
				"request_id", requestID,
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)
			err = status.Error(codes.Internal, "internal server error")
		}
		logRPCCompletion(ctx, "unary", info.FullMethod, requestID, startedAt, err)
	}()

	return handler(ctx, req)
}

func streamInterceptor(
	service any,
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	stream = &contextServerStream{ServerStream: stream, ctx: contextWithTraceID(stream.Context())}
	startedAt := time.Now()
	requestID := incomingRequestID(stream.Context())

	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(stream.Context(), "gRPC stream panic recovered",
				"rpc_method", info.FullMethod,
				"request_id", requestID,
				"panic", fmt.Sprint(recovered),
				"stack", string(debug.Stack()),
			)
			err = status.Error(codes.Internal, "internal server error")
		}
		logRPCCompletion(stream.Context(), "stream", info.FullMethod, requestID, startedAt, err)
	}()

	return handler(service, stream)
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context {
	return s.ctx
}

func contextWithTraceID(ctx context.Context) context.Context {
	values := metadata.ValueFromIncomingContext(ctx, apptrace.MetadataKey)
	if len(values) > 0 {
		if traceID := apptrace.Normalize(values[0]); traceID != "" {
			return apptrace.ContextWithID(ctx, traceID)
		}
	}
	traceID, err := apptrace.NewID()
	if err != nil {
		return ctx
	}
	return apptrace.ContextWithID(ctx, traceID)
}

func logRPCCompletion(ctx context.Context, rpcType, method, requestID string, startedAt time.Time, err error) {
	code := status.Code(err)
	attributes := []any{
		"rpc_type", rpcType,
		"rpc_method", method,
		"grpc_code", code.String(),
		"duration_ms", float64(time.Since(startedAt).Microseconds()) / 1000,
	}
	if requestID != "" {
		attributes = append(attributes, "request_id", requestID)
	}
	if err != nil {
		attributes = append(attributes, "error", err)
		slog.WarnContext(ctx, "gRPC request completed", attributes...)
		return
	}
	slog.InfoContext(ctx, "gRPC request completed", attributes...)
}

func incomingRequestID(ctx context.Context) string {
	values := metadata.ValueFromIncomingContext(ctx, requestIDMetadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
