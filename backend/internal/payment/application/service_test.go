package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
)

type repositoryStub struct {
	payment       *domain.Payment
	allocations   []domain.Allocation
	feeBPS        int32
	platformOwner string
}

type gatewayStub struct {
	result domain.GatewayResult
}

func (g *gatewayStub) Name() string { return domain.ProviderVNPay }
func (g *gatewayStub) CreateCheckout(context.Context, domain.CheckoutRequest) (domain.Checkout, error) {
	return domain.Checkout{ProviderReference: "provider-ref", URL: "https://pay.example.test", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (g *gatewayStub) ParseWebhook(context.Context, map[string]string) (domain.GatewayResult, error) {
	return g.result, nil
}
func (g *gatewayStub) Query(context.Context, *domain.Payment) (domain.GatewayResult, error) {
	return g.result, nil
}
func (g *gatewayStub) Refund(context.Context, domain.RefundRequest) (domain.GatewayResult, error) {
	return g.result, nil
}

func (r *repositoryStub) CreateWallet(context.Context, *domain.Wallet) (*domain.Wallet, error) {
	return nil, nil
}
func (r *repositoryStub) FindWallet(context.Context, string) (*domain.Wallet, error) {
	return nil, domain.ErrWalletNotFound
}
func (r *repositoryStub) AdjustBalance(
	context.Context, string, int64, string, string, string, time.Time,
) (*domain.Wallet, error) {
	return nil, nil
}
func (r *repositoryStub) CreatePayment(
	_ context.Context,
	payment *domain.Payment,
	allocations []domain.Allocation,
	platformOwner string,
	feeBPS int32,
) (*domain.Payment, error) {
	r.payment, r.allocations, r.platformOwner, r.feeBPS = payment, allocations, platformOwner, feeBPS
	return payment, nil
}
func (r *repositoryStub) CreateGatewayPayment(
	_ context.Context, payment *domain.Payment, allocations []domain.Allocation,
) (*domain.Payment, error) {
	r.payment, r.allocations = payment, allocations
	return payment, nil
}
func (r *repositoryStub) ApplyGatewayResult(
	context.Context, domain.GatewayResult, string, string, int32, time.Time,
) (*domain.Payment, bool, error) {
	return r.payment, false, nil
}
func (r *repositoryStub) FindPayment(context.Context, string, string) (*domain.Payment, error) {
	return nil, domain.ErrPaymentNotFound
}
func (r *repositoryStub) FindPaymentInternal(context.Context, string) (*domain.Payment, error) {
	if r.payment == nil {
		return nil, domain.ErrPaymentNotFound
	}
	return r.payment, nil
}
func (r *repositoryStub) FindPaymentByOrder(context.Context, string, string) (*domain.Payment, error) {
	return nil, domain.ErrPaymentNotFound
}
func (r *repositoryStub) ListPendingGatewayPayments(context.Context, time.Time, int) ([]*domain.Payment, error) {
	return nil, nil
}
func (r *repositoryStub) RecordReconciliation(context.Context, domain.Reconciliation) error {
	return nil
}
func (r *repositoryStub) MarkRefundPending(context.Context, string, time.Time) (*domain.Payment, error) {
	r.payment.Status = domain.StatusRefundPending
	return r.payment, nil
}
func (r *repositoryStub) RefundPayment(
	context.Context, string, string, string, time.Time,
) (*domain.Payment, error) {
	return nil, nil
}

func TestCreatePaymentPassesValidatedLedgerAllocation(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, Config{
		Currency: "vnd", PlatformOwnerID: "platform", FundingOwnerID: "system:funding", PlatformFeeBPS: 1000,
	})
	buyerID := uuid.NewString()
	sellerID := uuid.NewString()

	payment, err := service.CreatePayment(
		context.Background(), uuid.NewString(), buyerID, 10000,
		[]domain.Allocation{{SellerID: sellerID, AmountCents: 10000}}, "payment-key",
		CreatePaymentOptions{},
	)
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if payment.Currency != "VND" || repository.feeBPS != 1000 || repository.platformOwner != "platform" {
		t.Fatalf("unexpected payment config: payment=%+v repository=%+v", payment, repository)
	}
}

func TestCreatePaymentRejectsSelfPurchase(t *testing.T) {
	service := NewService(&repositoryStub{}, Config{Currency: "VND", PlatformOwnerID: "platform"})
	buyerID := uuid.NewString()
	_, err := service.CreatePayment(
		context.Background(), uuid.NewString(), buyerID, 10000,
		[]domain.Allocation{{SellerID: buyerID, AmountCents: 10000}}, "payment-key",
		CreatePaymentOptions{},
	)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("CreatePayment() error = %v, want invalid input", err)
	}
}

func TestCreateGatewayPaymentReturnsPendingCheckout(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, Config{
		Currency: "VND", PlatformOwnerID: "platform", DefaultProvider: domain.ProviderWallet,
		PlatformFeeBPS: 1000,
	})
	service.SetGateway(&gatewayStub{})
	payment, err := service.CreatePayment(
		context.Background(), uuid.NewString(), uuid.NewString(), 10000,
		[]domain.Allocation{{SellerID: uuid.NewString(), AmountCents: 10000}}, "gateway-key",
		CreatePaymentOptions{Provider: domain.ProviderVNPay, ClientIP: "127.0.0.1", Locale: "vn"},
	)
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if payment.Status != domain.StatusPending || payment.Provider != domain.ProviderVNPay || payment.CheckoutURL == "" {
		t.Fatalf("unexpected gateway payment: %+v", payment)
	}
	if payment.PlatformFeeCents != 1000 {
		t.Fatalf("platform fee = %d, want 1000", payment.PlatformFeeCents)
	}
}
