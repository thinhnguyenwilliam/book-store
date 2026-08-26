package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidInput      = errors.New("invalid payment input")
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrPaymentNotFound   = errors.New("payment not found")
	ErrInsufficientFunds = errors.New("insufficient wallet balance")
	ErrPaymentState      = errors.New("invalid payment state")
	ErrIdempotency       = errors.New("idempotency key conflict")
	ErrProviderDisabled  = errors.New("payment provider is not configured")
	ErrInvalidSignature  = errors.New("invalid payment webhook signature")
	ErrAmountMismatch    = errors.New("payment amount mismatch")
	ErrProviderReference = errors.New("payment provider reference not found")
)

const (
	ProviderWallet = "wallet"
	ProviderVNPay  = "vnpay"

	StatusPending       = "pending"
	StatusSucceeded     = "succeeded"
	StatusFailed        = "failed"
	StatusRefundPending = "refund_pending"
	StatusRefunded      = "refunded"
)

type Wallet struct {
	ID            string
	OwnerID       string
	BalanceCents  int64
	Currency      string
	AllowNegative bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (w *Wallet) Normalize() {
	w.OwnerID = strings.TrimSpace(w.OwnerID)
	w.Currency = strings.ToUpper(strings.TrimSpace(w.Currency))
}

type Allocation struct {
	SellerID    string
	AmountCents int64
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
	IdempotencyKey        string
	Provider              string
	ProviderReference     string
	ProviderTransactionID string
	CheckoutURL           string
	ExpiresAt             time.Time
	PaidAt                time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CheckoutRequest struct {
	PaymentID   string
	OrderID     string
	AmountCents int64
	Currency    string
	ClientIP    string
	Locale      string
	BankCode    string
	CreatedAt   time.Time
}

type Checkout struct {
	ProviderReference string
	URL               string
	ExpiresAt         time.Time
}

type GatewayResult struct {
	Provider              string
	EventID               string
	ProviderReference     string
	ProviderTransactionID string
	Status                string
	AmountCents           int64
	OccurredAt            time.Time
	RawPayload            []byte
}

type RefundRequest struct {
	Payment        *Payment
	IdempotencyKey string
	Reason         string
	CreatedBy      string
	CreatedAt      time.Time
}

type Reconciliation struct {
	ID                    string
	PaymentID             string
	Provider              string
	ProviderTransactionID string
	LocalStatus           string
	ProviderStatus        string
	ExpectedAmountCents   int64
	ProviderAmountCents   int64
	Matched               bool
	MismatchReason        string
	CheckedAt             time.Time
}

type ReconcileSummary struct {
	Checked    int32
	Updated    int32
	Mismatched int32
}

type WebhookResponse struct {
	ResponseCode string
	Message      string
	Payment      *Payment
}
