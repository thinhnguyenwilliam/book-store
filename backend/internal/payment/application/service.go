package application

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
)

type Config struct {
	Currency        string
	PlatformOwnerID string
	FundingOwnerID  string
	ClearingOwnerID string
	DefaultProvider string
	PlatformFeeBPS  int32
	ReconcileGrace  time.Duration
}

type Service struct {
	repository Repository
	config     Config
	gateways   map[string]Gateway
	now        func() time.Time
}

func NewService(repository Repository, config Config) *Service {
	config.Currency = strings.ToUpper(strings.TrimSpace(config.Currency))
	if config.Currency == "" {
		config.Currency = "VND"
	}
	if config.PlatformOwnerID == "" {
		config.PlatformOwnerID = "platform"
	}
	if config.FundingOwnerID == "" {
		config.FundingOwnerID = "system:funding"
	}
	if config.ClearingOwnerID == "" {
		config.ClearingOwnerID = "gateway:vnpay:clearing"
	}
	config.DefaultProvider = strings.ToLower(strings.TrimSpace(config.DefaultProvider))
	if config.DefaultProvider == "" {
		config.DefaultProvider = domain.ProviderWallet
	}
	if config.ReconcileGrace <= 0 {
		config.ReconcileGrace = 2 * time.Minute
	}
	return &Service{repository: repository, config: config, gateways: make(map[string]Gateway), now: time.Now}
}

func (s *Service) SetGateway(gateway Gateway) {
	if gateway != nil {
		s.gateways[gateway.Name()] = gateway
	}
}

type CreatePaymentOptions struct {
	Provider string
	ClientIP string
	Locale   string
	BankCode string
}

func (s *Service) CreateWallet(ctx context.Context, ownerID string) (*domain.Wallet, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	return s.repository.CreateWallet(ctx, &domain.Wallet{
		ID:        uuid.NewString(),
		OwnerID:   ownerID,
		Currency:  s.config.Currency,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Service) GetBalance(ctx context.Context, ownerID string) (*domain.Wallet, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindWallet(ctx, ownerID)
}

func (s *Service) UpdateBalance(
	ctx context.Context,
	ownerID string,
	deltaCents int64,
	idempotencyKey, reason string,
) (*domain.Wallet, error) {
	if strings.TrimSpace(ownerID) == "" || deltaCents == 0 || deltaCents == math.MinInt64 || strings.TrimSpace(idempotencyKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.AdjustBalance(
		ctx,
		ownerID,
		deltaCents,
		idempotencyKey,
		strings.TrimSpace(reason),
		s.config.FundingOwnerID,
		s.now().UTC(),
	)
}

func (s *Service) CreatePayment(
	ctx context.Context,
	orderID, buyerID string,
	totalCents int64,
	allocations []domain.Allocation,
	idempotencyKey string,
	options CreatePaymentOptions,
) (*domain.Payment, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(buyerID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if totalCents <= 0 || strings.TrimSpace(idempotencyKey) == "" || len(allocations) == 0 {
		return nil, domain.ErrInvalidInput
	}
	allocationBySeller := make(map[string]int64, len(allocations))
	var allocationTotal int64
	for index := range allocations {
		allocations[index].SellerID = strings.TrimSpace(allocations[index].SellerID)
		if allocations[index].SellerID == "" {
			allocations[index].SellerID = s.config.PlatformOwnerID
		}
		if allocations[index].AmountCents <= 0 {
			return nil, domain.ErrInvalidInput
		}
		if allocations[index].SellerID == buyerID {
			return nil, domain.ErrInvalidInput
		}
		if allocationTotal > math.MaxInt64-allocations[index].AmountCents {
			return nil, domain.ErrInvalidInput
		}
		allocationTotal += allocations[index].AmountCents
		current := allocationBySeller[allocations[index].SellerID]
		if current > math.MaxInt64-allocations[index].AmountCents {
			return nil, domain.ErrInvalidInput
		}
		allocationBySeller[allocations[index].SellerID] = current + allocations[index].AmountCents
	}
	if allocationTotal != totalCents || s.config.PlatformFeeBPS < 0 || s.config.PlatformFeeBPS > 10000 {
		return nil, domain.ErrInvalidInput
	}
	normalizedAllocations := make([]domain.Allocation, 0, len(allocationBySeller))
	for sellerID, amount := range allocationBySeller {
		normalizedAllocations = append(normalizedAllocations, domain.Allocation{
			SellerID: sellerID, AmountCents: amount,
		})
	}
	now := s.now().UTC()
	provider := strings.ToLower(strings.TrimSpace(options.Provider))
	if provider == "" {
		provider = s.config.DefaultProvider
	}
	if provider == domain.ProviderWallet {
		return s.repository.CreatePayment(ctx, &domain.Payment{
			ID:             uuid.NewString(),
			OrderID:        orderID,
			BuyerID:        buyerID,
			Status:         domain.StatusSucceeded,
			AmountCents:    totalCents,
			Currency:       s.config.Currency,
			IdempotencyKey: idempotencyKey,
			Provider:       domain.ProviderWallet,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, normalizedAllocations, s.config.PlatformOwnerID, s.config.PlatformFeeBPS)
	}
	gateway, ok := s.gateways[provider]
	if !ok {
		return nil, domain.ErrProviderDisabled
	}
	payment := &domain.Payment{
		ID: uuid.NewString(), OrderID: orderID, BuyerID: buyerID, Status: domain.StatusPending,
		AmountCents: totalCents, Currency: s.config.Currency, IdempotencyKey: idempotencyKey,
		Provider: provider, CreatedAt: now, UpdatedAt: now,
	}
	for _, allocation := range normalizedAllocations {
		payment.PlatformFeeCents += gatewayFeeCents(allocation.AmountCents, s.config.PlatformFeeBPS)
	}
	checkout, err := gateway.CreateCheckout(ctx, domain.CheckoutRequest{
		PaymentID: payment.ID, OrderID: orderID, AmountCents: totalCents, Currency: s.config.Currency,
		ClientIP: options.ClientIP, Locale: options.Locale, BankCode: options.BankCode, CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	payment.ProviderReference = checkout.ProviderReference
	payment.CheckoutURL = checkout.URL
	payment.ExpiresAt = checkout.ExpiresAt
	return s.repository.CreateGatewayPayment(ctx, payment, normalizedAllocations)
}

func (s *Service) ProcessWebhook(
	ctx context.Context,
	provider string,
	parameters map[string]string,
) (domain.WebhookResponse, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	gateway, ok := s.gateways[provider]
	if !ok {
		return domain.WebhookResponse{ResponseCode: "99", Message: "Provider not configured"}, domain.ErrProviderDisabled
	}
	result, err := gateway.ParseWebhook(ctx, parameters)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidSignature) {
			return domain.WebhookResponse{ResponseCode: "97", Message: "Invalid signature"}, nil
		}
		return domain.WebhookResponse{ResponseCode: "99", Message: "Invalid payload"}, err
	}
	payment, duplicate, err := s.repository.ApplyGatewayResult(
		ctx, result, s.config.PlatformOwnerID, s.config.ClearingOwnerID,
		s.config.PlatformFeeBPS, s.now().UTC(),
	)
	if errors.Is(err, domain.ErrProviderReference) {
		return domain.WebhookResponse{ResponseCode: "01", Message: "Order not found"}, nil
	}
	if errors.Is(err, domain.ErrAmountMismatch) {
		return domain.WebhookResponse{ResponseCode: "04", Message: "Invalid amount"}, nil
	}
	if err != nil {
		return domain.WebhookResponse{ResponseCode: "99", Message: "Processing error"}, err
	}
	if duplicate {
		return domain.WebhookResponse{ResponseCode: "02", Message: "Order already confirmed", Payment: payment}, nil
	}
	return domain.WebhookResponse{ResponseCode: "00", Message: "Confirm Success", Payment: payment}, nil
}

func (s *Service) ReconcilePendingPayments(ctx context.Context, limit int) (domain.ReconcileSummary, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	payments, err := s.repository.ListPendingGatewayPayments(ctx, s.now().UTC().Add(-s.config.ReconcileGrace), limit)
	if err != nil {
		return domain.ReconcileSummary{}, err
	}
	var summary domain.ReconcileSummary
	var resultErr error
	for _, payment := range payments {
		summary.Checked++
		gateway, ok := s.gateways[payment.Provider]
		if !ok {
			continue
		}
		result, queryErr := gateway.Query(ctx, payment)
		if queryErr != nil {
			resultErr = errors.Join(resultErr, queryErr)
			continue
		}
		matched := result.AmountCents == payment.AmountCents && result.ProviderReference == payment.ProviderReference
		mismatch := ""
		if !matched {
			mismatch = "provider reference or amount mismatch"
			summary.Mismatched++
		}
		reconciliation := domain.Reconciliation{
			ID: uuid.NewString(), PaymentID: payment.ID, Provider: payment.Provider,
			ProviderTransactionID: result.ProviderTransactionID, LocalStatus: payment.Status,
			ProviderStatus: result.Status, ExpectedAmountCents: payment.AmountCents,
			ProviderAmountCents: result.AmountCents, Matched: matched,
			MismatchReason: mismatch, CheckedAt: s.now().UTC(),
		}
		if recordErr := s.repository.RecordReconciliation(ctx, reconciliation); recordErr != nil {
			resultErr = errors.Join(resultErr, recordErr)
			continue
		}
		shouldApply := payment.Status == domain.StatusPending && result.Status != domain.StatusPending
		shouldApply = shouldApply || payment.Status == domain.StatusRefundPending && result.Status == domain.StatusRefunded
		if matched && shouldApply {
			if _, _, applyErr := s.repository.ApplyGatewayResult(
				ctx, result, s.config.PlatformOwnerID, s.config.ClearingOwnerID,
				s.config.PlatformFeeBPS, s.now().UTC(),
			); applyErr != nil {
				resultErr = errors.Join(resultErr, applyErr)
			} else {
				summary.Updated++
			}
		}
	}
	return summary, resultErr
}

func gatewayFeeCents(amount int64, feeBPS int32) int64 {
	fee := int64(feeBPS)
	return amount/10000*fee + amount%10000*fee/10000
}

func (s *Service) GetPayment(ctx context.Context, id, buyerID string) (*domain.Payment, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(buyerID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindPayment(ctx, id, buyerID)
}

func (s *Service) GetPaymentByOrder(ctx context.Context, orderID, buyerID string) (*domain.Payment, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if _, err := uuid.Parse(buyerID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindPaymentByOrder(ctx, orderID, buyerID)
}

func (s *Service) RefundPayment(
	ctx context.Context,
	paymentID, idempotencyKey, reason string,
) (*domain.Payment, error) {
	if _, err := uuid.Parse(paymentID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	reason = strings.TrimSpace(reason)
	now := s.now().UTC()
	payment, err := s.repository.FindPaymentInternal(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if payment.Status == domain.StatusRefunded {
		return payment, nil
	}
	if payment.Provider == domain.ProviderWallet {
		return s.repository.RefundPayment(ctx, paymentID, idempotencyKey, reason, now)
	}
	gateway, ok := s.gateways[payment.Provider]
	if !ok {
		return nil, domain.ErrProviderDisabled
	}
	if payment.Status == domain.StatusRefundPending {
		result, queryErr := gateway.Query(ctx, payment)
		if queryErr != nil {
			return nil, queryErr
		}
		if result.Status != domain.StatusRefunded {
			return payment, nil
		}
		return s.repository.RefundPayment(ctx, paymentID, idempotencyKey, reason, now)
	}
	if payment.Status != domain.StatusSucceeded {
		return nil, domain.ErrPaymentState
	}
	result, err := gateway.Refund(ctx, domain.RefundRequest{
		Payment: payment, IdempotencyKey: idempotencyKey, Reason: reason,
		CreatedBy: "payment-service", CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	switch result.Status {
	case domain.StatusRefunded:
		return s.repository.RefundPayment(ctx, paymentID, idempotencyKey, reason, now)
	case domain.StatusRefundPending:
		return s.repository.MarkRefundPending(ctx, paymentID, now)
	default:
		return nil, domain.ErrPaymentState
	}
}
