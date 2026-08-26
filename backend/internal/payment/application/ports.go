package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
)

type Repository interface {
	CreateWallet(ctx context.Context, wallet *domain.Wallet) (*domain.Wallet, error)
	FindWallet(ctx context.Context, ownerID string) (*domain.Wallet, error)
	AdjustBalance(
		ctx context.Context,
		ownerID string,
		deltaCents int64,
		idempotencyKey, reason, fundingOwnerID string,
		now time.Time,
	) (*domain.Wallet, error)
	CreatePayment(
		ctx context.Context,
		payment *domain.Payment,
		allocations []domain.Allocation,
		platformOwnerID string,
		platformFeeBPS int32,
	) (*domain.Payment, error)
	CreateGatewayPayment(ctx context.Context, payment *domain.Payment, allocations []domain.Allocation) (*domain.Payment, error)
	ApplyGatewayResult(
		ctx context.Context,
		result domain.GatewayResult,
		platformOwnerID, clearingOwnerID string,
		platformFeeBPS int32,
		now time.Time,
	) (*domain.Payment, bool, error)
	FindPayment(ctx context.Context, id, buyerID string) (*domain.Payment, error)
	FindPaymentInternal(ctx context.Context, id string) (*domain.Payment, error)
	FindPaymentByOrder(ctx context.Context, orderID, buyerID string) (*domain.Payment, error)
	ListPendingGatewayPayments(ctx context.Context, before time.Time, limit int) ([]*domain.Payment, error)
	RecordReconciliation(ctx context.Context, reconciliation domain.Reconciliation) error
	MarkRefundPending(ctx context.Context, paymentID string, now time.Time) (*domain.Payment, error)
	RefundPayment(
		ctx context.Context,
		paymentID, idempotencyKey, reason string,
		now time.Time,
	) (*domain.Payment, error)
}

type Gateway interface {
	Name() string
	CreateCheckout(ctx context.Context, request domain.CheckoutRequest) (domain.Checkout, error)
	ParseWebhook(ctx context.Context, parameters map[string]string) (domain.GatewayResult, error)
	Query(ctx context.Context, payment *domain.Payment) (domain.GatewayResult, error)
	Refund(ctx context.Context, request domain.RefundRequest) (domain.GatewayResult, error)
}
