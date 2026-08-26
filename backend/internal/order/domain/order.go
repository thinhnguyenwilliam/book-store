package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput        = errors.New("invalid order input")
	ErrCartItemNotFound    = errors.New("cart item not found")
	ErrCartEmpty           = errors.New("cart is empty")
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderState          = errors.New("invalid order state")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
	ErrPaymentDeclined     = errors.New("payment declined")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrRefundPending       = errors.New("payment refund is pending")
)

const (
	StatusPending             = "pending"
	StatusStockReserved       = "stock_reserved"
	StatusPaymentPending      = "payment_pending"
	StatusConfirmed           = "confirmed"
	StatusCancelled           = "cancelled"
	StatusCompensationPending = "compensation_pending"
)

type CartItem struct {
	ID        string
	UserID    string
	BookID    string
	Quantity  int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Item struct {
	ID             string
	BookID         string
	SellerID       string
	Title          string
	UnitPriceCents int64
	Quantity       int32
	SubtotalCents  int64
}

type Order struct {
	ID                   string
	UserID               string
	Status               string
	TotalCents           int64
	Currency             string
	Items                []Item
	PaymentID            string
	FailureReason        string
	IdempotencyKey       string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ReservationExpiresAt time.Time
}

type Page struct {
	Orders     []*Order
	NextCursor string
	HasMore    bool
}

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type BookSnapshot struct {
	ID         string
	SellerID   string
	Title      string
	PriceCents int64
}

type Payment struct {
	ID                    string
	OrderID               string
	BuyerID               string
	Status                string
	AmountCents           int64
	PlatformFeeCents      int64
	Currency              string
	FailureReason         string
	Provider              string
	ProviderTransactionID string
	CheckoutURL           string
	ExpiresAt             time.Time
	PaidAt                time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type PaymentOptions struct {
	Provider string
	ClientIP string
	Locale   string
	BankCode string
}

type PaymentAllocation struct {
	SellerID    string
	AmountCents int64
}
