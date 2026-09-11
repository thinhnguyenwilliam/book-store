package grpcerror

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WithReason carries a stable application reason across service boundaries.
func WithReason(code codes.Code, reason, message string) error {
	s := status.New(code, message)
	detailed, err := s.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "bookstore"})
	if err != nil {
		return s.Err()
	}
	return detailed.Err()
}

func Reason(err error) string {
	for _, detail := range status.Convert(err).Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "bookstore" {
			return info.Reason
		}
	}
	return ""
}
