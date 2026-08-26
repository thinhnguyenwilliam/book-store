package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
)

type Repository interface {
	AddCartItem(ctx context.Context, item *domain.CartItem) (*domain.CartItem, error)
	UpdateCartItem(ctx context.Context, userID, itemID string, quantity int32, now time.Time) (*domain.CartItem, error)
	RemoveCartItems(ctx context.Context, userID string, itemIDs []string) error
	ListCart(ctx context.Context, userID string) ([]*domain.CartItem, error)
	ClearCart(ctx context.Context, userID string) error
	CreateOrder(ctx context.Context, order *domain.Order) error
	FindOrder(ctx context.Context, userID, id string) (*domain.Order, error)
	FindOrderByIdempotency(ctx context.Context, userID, idempotencyKey string) (*domain.Order, error)
	ListOrders(ctx context.Context, userID string, limit int32, cursor *domain.Cursor) ([]*domain.Order, error)
	UpdateOrderState(
		ctx context.Context,
		userID, id string,
		allowedStatuses []string,
		status, paymentID, failureReason string,
		now time.Time,
	) (*domain.Order, error)
	ListOrdersForReconciliation(
		ctx context.Context, now, paymentCutoff time.Time, limit int,
	) ([]*domain.Order, error)
}

type BookClient interface {
	GetBook(ctx context.Context, bookID string) (domain.BookSnapshot, error)
	ReserveStock(ctx context.Context, orderID, bookID string, quantity int32, idempotencyKey string) error
	CommitStock(ctx context.Context, orderID, bookID string) error
	ReleaseStock(ctx context.Context, orderID, bookID string) error
}

type PaymentClient interface {
	CreatePayment(
		ctx context.Context,
		orderID, buyerID string,
		totalCents int64,
		allocations []domain.PaymentAllocation,
		idempotencyKey string,
		options domain.PaymentOptions,
	) (*domain.Payment, error)
	GetPaymentByOrder(ctx context.Context, orderID, buyerID string) (*domain.Payment, error)
	RefundPayment(ctx context.Context, paymentID, idempotencyKey, reason string) (*domain.Payment, error)
}

type Cache interface {
	GetJSON(ctx context.Context, key string, destination any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Version(ctx context.Context, key string) (int64, error)
	BumpVersion(ctx context.Context, key string) error
	TryLock(ctx context.Context, key string, ttl time.Duration) (token string, locked bool, err error)
	Unlock(ctx context.Context, key, token string) error
}
