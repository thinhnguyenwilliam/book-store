package grpcerror

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFromContext(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantOK   bool
		wantCode codes.Code
	}{
		{name: "canceled", err: fmt.Errorf("repository: %w", context.Canceled), wantOK: true, wantCode: codes.Canceled},
		{name: "deadline", err: fmt.Errorf("repository: %w", context.DeadlineExceeded), wantOK: true, wantCode: codes.DeadlineExceeded},
		{name: "domain error", err: fmt.Errorf("invalid input"), wantOK: false, wantCode: codes.OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := FromContext(tt.err)
			ok := mapped != nil
			if ok != tt.wantOK {
				t.Fatalf("ok = %t, want %t", ok, tt.wantOK)
			}
			if ok && status.Code(mapped) != tt.wantCode {
				t.Fatalf("code = %s, want %s", status.Code(mapped), tt.wantCode)
			}
		})
	}
}
