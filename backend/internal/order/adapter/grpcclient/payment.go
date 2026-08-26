package grpcclient

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentClient struct {
	client bookstorev1.PaymentServiceClient
}

func NewPaymentClient(client bookstorev1.PaymentServiceClient) *PaymentClient {
	return &PaymentClient{client: client}
}

func (c *PaymentClient) CreatePayment(
	ctx context.Context,
	orderID, buyerID string,
	totalCents int64,
	allocations []domain.PaymentAllocation,
	idempotencyKey string,
	options domain.PaymentOptions,
) (*domain.Payment, error) {
	protoAllocations := make([]*bookstorev1.PaymentAllocation, 0, len(allocations))
	for _, allocation := range allocations {
		protoAllocations = append(protoAllocations, &bookstorev1.PaymentAllocation{
			SellerId: allocation.SellerID, AmountCents: allocation.AmountCents,
		})
	}
	payment, err := c.client.CreatePayment(ctx, &bookstorev1.CreatePaymentRequest{
		OrderId: orderID, BuyerId: buyerID, TotalCents: totalCents,
		Allocations: protoAllocations, IdempotencyKey: idempotencyKey,
		Provider: options.Provider, ClientIp: options.ClientIP,
		Locale: options.Locale, BankCode: options.BankCode,
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			return nil, errors.Join(domain.ErrPaymentDeclined, err)
		}
		return nil, err
	}
	return paymentDomain(payment), nil
}

func (c *PaymentClient) GetPaymentByOrder(
	ctx context.Context,
	orderID, buyerID string,
) (*domain.Payment, error) {
	payment, err := c.client.GetPaymentByOrder(ctx, &bookstorev1.GetPaymentByOrderRequest{
		OrderId: orderID, BuyerId: buyerID,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.Join(domain.ErrPaymentNotFound, err)
		}
		return nil, err
	}
	return paymentDomain(payment), nil
}

func (c *PaymentClient) RefundPayment(
	ctx context.Context,
	paymentID, idempotencyKey, reason string,
) (*domain.Payment, error) {
	payment, err := c.client.RefundPayment(ctx, &bookstorev1.RefundPaymentRequest{
		PaymentId: paymentID, IdempotencyKey: idempotencyKey, Reason: reason,
	})
	if err != nil {
		return nil, err
	}
	return paymentDomain(payment), nil
}

func paymentDomain(payment *bookstorev1.Payment) *domain.Payment {
	createdAt, _ := time.Parse(time.RFC3339, payment.GetCreatedAt())
	updatedAt, _ := time.Parse(time.RFC3339, payment.GetUpdatedAt())
	return &domain.Payment{
		ID: payment.GetId(), OrderID: payment.GetOrderId(), BuyerID: payment.GetBuyerId(),
		Status: payment.GetStatus(), AmountCents: payment.GetAmountCents(),
		PlatformFeeCents: payment.GetPlatformFeeCents(), Currency: payment.GetCurrency(),
		FailureReason: payment.GetFailureReason(), CreatedAt: createdAt, UpdatedAt: updatedAt,
		Provider: payment.GetProvider(), ProviderTransactionID: payment.GetProviderTransactionId(),
		CheckoutURL: payment.GetCheckoutUrl(), ExpiresAt: parseOptionalTime(payment.GetExpiresAt()),
		PaidAt: parseOptionalTime(payment.GetPaidAt()),
	}
}

func parseOptionalTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}
