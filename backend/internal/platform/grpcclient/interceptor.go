package grpcclient

import (
	"context"
	"log/slog"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func UnaryLoggingInterceptor(
	ctx context.Context,
	method string,
	req any,
	reply any,
	connection *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	options ...grpc.CallOption,
) error {
	ctx = contextWithOutgoingTraceID(ctx)
	startedAt := time.Now()
	err := invoker(ctx, method, req, reply, connection, options...)
	logCompletion(ctx, "unary", method, startedAt, err)
	return err
}

func StreamLoggingInterceptor(
	ctx context.Context,
	description *grpc.StreamDesc,
	connection *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	options ...grpc.CallOption,
) (grpc.ClientStream, error) {
	ctx = contextWithOutgoingTraceID(ctx)
	startedAt := time.Now()
	stream, err := streamer(ctx, description, connection, method, options...)
	logCompletion(ctx, "stream_open", method, startedAt, err)
	return stream, err
}

func contextWithOutgoingTraceID(ctx context.Context) context.Context {
	traceID := apptrace.IDFromContext(ctx)
	if traceID == "" {
		var err error
		traceID, err = apptrace.NewID()
		if err != nil {
			return ctx
		}
		ctx = apptrace.ContextWithID(ctx, traceID)
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(apptrace.MetadataKey, traceID)
	return metadata.NewOutgoingContext(ctx, md)
}

func logCompletion(ctx context.Context, rpcType, method string, startedAt time.Time, err error) {
	attributes := []any{
		"rpc_type", rpcType,
		"rpc_method", method,
		"grpc_code", status.Code(err).String(),
		"duration_ms", float64(time.Since(startedAt).Microseconds()) / 1000,
	}
	if requestID := outgoingRequestID(ctx); requestID != "" {
		attributes = append(attributes, "request_id", requestID)
	}
	if err != nil {
		attributes = append(attributes, "error", err)
		slog.WarnContext(ctx, "gRPC client call completed", attributes...)
		return
	}
	slog.InfoContext(ctx, "gRPC client call completed", attributes...)
}

func outgoingRequestID(ctx context.Context) string {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("x-request-id")
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
