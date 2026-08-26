package grpcerror

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FromContext preserves cancellation semantics at the gRPC delivery boundary.
// Without this check, a repository or external provider timeout can be hidden
// behind a generic Internal status by a domain-error switch.
func FromContext(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	default:
		return nil
	}
}
