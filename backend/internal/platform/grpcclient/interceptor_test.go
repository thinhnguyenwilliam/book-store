package grpcclient

import (
	"context"
	"testing"
	"time"

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

func TestUnaryDeadlineInterceptorAddsDefaultDeadline(t *testing.T) {
	timeout := 200 * time.Millisecond
	interceptor := UnaryDeadlineInterceptor(timeout)
	err := interceptor(
		context.Background(),
		"/bookstore.v1.Test/Deadline",
		nil,
		nil,
		nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("gRPC context deadline is missing")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > timeout {
				t.Fatalf("remaining deadline = %s, want within (0, %s]", remaining, timeout)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("UnaryDeadlineInterceptor() error = %v", err)
	}
}

func TestUnaryDeadlineInterceptorPreservesEarlierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	wantDeadline, _ := ctx.Deadline()

	interceptor := UnaryDeadlineInterceptor(time.Second)
	err := interceptor(
		ctx,
		"/bookstore.v1.Test/Deadline",
		nil,
		nil,
		nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			gotDeadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("gRPC context deadline is missing")
			}
			if !gotDeadline.Equal(wantDeadline) {
				t.Fatalf("deadline = %s, want parent deadline %s", gotDeadline, wantDeadline)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("UnaryDeadlineInterceptor() error = %v", err)
	}
}
