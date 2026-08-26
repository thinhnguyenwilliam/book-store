package grpc

import (
	"context"
	"errors"
	"time"

	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
	grpcerror "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/grpcerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	bookstorev1.UnimplementedPaymentServiceServer
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateWallet(
	ctx context.Context,
	request *bookstorev1.CreateWalletRequest,
) (*bookstorev1.Wallet, error) {
	wallet, err := h.service.CreateWallet(ctx, request.GetOwnerId())
	if err != nil {
		return nil, mapError(err)
	}
	return walletProto(wallet), nil
}

func (h *Handler) GetBalance(
	ctx context.Context,
	request *bookstorev1.GetBalanceRequest,
) (*bookstorev1.Wallet, error) {
	wallet, err := h.service.GetBalance(ctx, request.GetOwnerId())
	if err != nil {
		return nil, mapError(err)
	}
	return walletProto(wallet), nil
}

func (h *Handler) UpdateBalance(
	ctx context.Context,
	request *bookstorev1.UpdateBalanceRequest,
) (*bookstorev1.Wallet, error) {
	wallet, err := h.service.UpdateBalance(
		ctx,
		request.GetOwnerId(),
		request.GetDeltaCents(),
		request.GetIdempotencyKey(),
		request.GetReason(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return walletProto(wallet), nil
}

func (h *Handler) CreatePayment(
	ctx context.Context,
	request *bookstorev1.CreatePaymentRequest,
) (*bookstorev1.Payment, error) {
	allocations := make([]domain.Allocation, 0, len(request.GetAllocations()))
	for _, allocation := range request.GetAllocations() {
		allocations = append(allocations, domain.Allocation{
			SellerID: allocation.GetSellerId(), AmountCents: allocation.GetAmountCents(),
		})
	}
	payment, err := h.service.CreatePayment(
		ctx,
		request.GetOrderId(),
		request.GetBuyerId(),
		request.GetTotalCents(),
		allocations,
		request.GetIdempotencyKey(),
		application.CreatePaymentOptions{
			Provider: request.GetProvider(), ClientIP: request.GetClientIp(),
			Locale: request.GetLocale(), BankCode: request.GetBankCode(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}
	return paymentProto(payment), nil
}

func (h *Handler) ProcessWebhook(
	ctx context.Context,
	request *bookstorev1.ProcessPaymentWebhookRequest,
) (*bookstorev1.ProcessPaymentWebhookResponse, error) {
	result, err := h.service.ProcessWebhook(ctx, request.GetProvider(), request.GetParameters())
	if err != nil {
		return nil, mapError(err)
	}
	response := &bookstorev1.ProcessPaymentWebhookResponse{
		ResponseCode: result.ResponseCode, Message: result.Message,
	}
	if result.Payment != nil {
		response.Payment = paymentProto(result.Payment)
	}
	return response, nil
}

func (h *Handler) ReconcilePendingPayments(
	ctx context.Context,
	request *bookstorev1.ReconcilePendingPaymentsRequest,
) (*bookstorev1.ReconcilePendingPaymentsResponse, error) {
	summary, err := h.service.ReconcilePendingPayments(ctx, int(request.GetLimit()))
	if err != nil {
		return nil, mapError(err)
	}
	return &bookstorev1.ReconcilePendingPaymentsResponse{
		Checked: summary.Checked, Updated: summary.Updated, Mismatched: summary.Mismatched,
	}, nil
}

func (h *Handler) GetPayment(
	ctx context.Context,
	request *bookstorev1.GetPaymentRequest,
) (*bookstorev1.Payment, error) {
	payment, err := h.service.GetPayment(ctx, request.GetId(), request.GetBuyerId())
	if err != nil {
		return nil, mapError(err)
	}
	return paymentProto(payment), nil
}

func (h *Handler) GetPaymentByOrder(
	ctx context.Context,
	request *bookstorev1.GetPaymentByOrderRequest,
) (*bookstorev1.Payment, error) {
	payment, err := h.service.GetPaymentByOrder(ctx, request.GetOrderId(), request.GetBuyerId())
	if err != nil {
		return nil, mapError(err)
	}
	return paymentProto(payment), nil
}

func (h *Handler) RefundPayment(
	ctx context.Context,
	request *bookstorev1.RefundPaymentRequest,
) (*bookstorev1.Payment, error) {
	payment, err := h.service.RefundPayment(
		ctx,
		request.GetPaymentId(),
		request.GetIdempotencyKey(),
		request.GetReason(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return paymentProto(payment), nil
}

func walletProto(wallet *domain.Wallet) *bookstorev1.Wallet {
	return &bookstorev1.Wallet{
		Id: wallet.ID, OwnerId: wallet.OwnerID, BalanceCents: wallet.BalanceCents,
		Currency: wallet.Currency, CreatedAt: wallet.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wallet.UpdatedAt.Format(time.RFC3339),
	}
}

func paymentProto(payment *domain.Payment) *bookstorev1.Payment {
	return &bookstorev1.Payment{
		Id: payment.ID, OrderId: payment.OrderID, BuyerId: payment.BuyerID,
		Status: payment.Status, AmountCents: payment.AmountCents,
		PlatformFeeCents: payment.PlatformFeeCents, Currency: payment.Currency,
		FailureReason: payment.FailureReason, CreatedAt: payment.CreatedAt.Format(time.RFC3339),
		UpdatedAt: payment.UpdatedAt.Format(time.RFC3339),
		Provider:  payment.Provider, ProviderTransactionId: payment.ProviderTransactionID,
		CheckoutUrl: payment.CheckoutURL, ExpiresAt: formatOptionalTime(payment.ExpiresAt),
		PaidAt: formatOptionalTime(payment.PaidAt),
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func mapError(err error) error {
	if mapped := grpcerror.FromContext(err); mapped != nil {
		return mapped
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrWalletNotFound), errors.Is(err, domain.ErrPaymentNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrPaymentState):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrIdempotency):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrProviderDisabled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidSignature):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrAmountMismatch), errors.Is(err, domain.ErrProviderReference):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
