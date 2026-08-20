package grpcserver

import (
	"context"
	"testing"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryInterceptorRecoversPanic(t *testing.T) {
	_, err := unaryInterceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/bookstore.v1.Test/Panic"},
		func(context.Context, any) (any, error) { panic("boom") },
	)
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

func TestUnaryInterceptorPropagatesIncomingTraceID(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(apptrace.MetadataKey, traceID))
	_, err := unaryInterceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/bookstore.v1.Test/Trace"},
		func(ctx context.Context, _ any) (any, error) {
			if got := apptrace.IDFromContext(ctx); got != traceID {
				t.Fatalf("handler trace ID = %q, want %q", got, traceID)
			}
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unaryInterceptor() error = %v", err)
	}
}
