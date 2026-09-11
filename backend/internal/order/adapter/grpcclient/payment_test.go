package grpcclient

import (
	"context"
	"errors"
	"testing"

	pb "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type failingPaymentRPC struct {
	pb.PaymentServiceClient
	err error
}

func (c failingPaymentRPC) CreatePayment(context.Context, *pb.CreatePaymentRequest, ...grpc.CallOption) (*pb.Payment, error) {
	return nil, c.err
}

func TestWalletErrorsAreDefinitiveDeclines(t *testing.T) {
	for reason, want := range map[string]error{"WALLET_NOT_FOUND": domain.ErrWalletNotFound, "INSUFFICIENT_FUNDS": domain.ErrInsufficientFunds} {
		t.Run(reason, func(t *testing.T) {
			c := NewPaymentClient(failingPaymentRPC{err: grpcerror.WithReason(codes.NotFound, reason, "provider detail")})
			_, err := c.CreatePayment(context.Background(), "", "", 100, nil, "key", domain.PaymentOptions{})
			if !errors.Is(err, want) || !errors.Is(err, domain.ErrPaymentDeclined) {
				t.Fatalf("unexpected classification: %v", err)
			}
		})
	}
}
