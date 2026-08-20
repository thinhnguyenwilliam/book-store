package grpcclient

import (
	"context"
	"testing"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryInterceptorInjectsTraceID(t *testing.T) {
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := apptrace.ContextWithID(context.Background(), traceID)
	err := UnaryLoggingInterceptor(
		ctx,
		"/bookstore.v1.Test/Trace",
		nil,
		nil,
		nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("outgoing metadata is missing")
			}
			if got := md.Get(apptrace.MetadataKey); len(got) != 1 || got[0] != traceID {
				t.Fatalf("trace ID metadata = %v, want [%s]", got, traceID)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("UnaryLoggingInterceptor() error = %v", err)
	}
}
