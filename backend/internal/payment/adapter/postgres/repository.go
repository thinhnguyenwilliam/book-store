package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type walletModel struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	OwnerID       string    `gorm:"not null;uniqueIndex"`
	BalanceCents  int64     `gorm:"not null"`
	Currency      string    `gorm:"not null"`
	AllowNegative bool      `gorm:"not null"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

type paymentModel struct {
	ID                    string `gorm:"type:uuid;primaryKey"`
	OrderID               string `gorm:"type:uuid;not null;uniqueIndex"`
	BuyerID               string `gorm:"type:uuid;not null"`
	Status                string `gorm:"not null"`
	AmountCents           int64  `gorm:"not null"`
	PlatformFeeCents      int64  `gorm:"not null"`
	Currency              string `gorm:"not null"`
	FailureReason         string `gorm:"not null"`
	IdempotencyKey        string `gorm:"not null"`
	Provider              string `gorm:"not null"`
	ProviderReference     string `gorm:"not null"`
	ProviderTransactionID string `gorm:"not null"`
	CheckoutURL           string `gorm:"not null"`
	ExpiresAt             *time.Time
	PaidAt                *time.Time
	CreatedAt             time.Time `gorm:"not null"`
	UpdatedAt             time.Time `gorm:"not null"`
}

type paymentAllocationModel struct {
	PaymentID   string    `gorm:"type:uuid;primaryKey"`
	SellerID    string    `gorm:"primaryKey"`
	AmountCents int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

type webhookEventModel struct {
	ID                string `gorm:"type:uuid;primaryKey"`
	Provider          string `gorm:"not null"`
	EventID           string `gorm:"not null"`
	ProviderReference string `gorm:"not null"`
	SignatureValid    bool   `gorm:"not null"`
	Payload           []byte `gorm:"type:jsonb;not null"`
	ProcessingError   string `gorm:"not null"`
	ProcessedAt       *time.Time
	ReceivedAt        time.Time `gorm:"not null"`
}

type paymentOutboxModel struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	AggregateID string    `gorm:"type:uuid;not null"`
	EventType   string    `gorm:"not null"`
	TraceID     string    `gorm:"not null"`
	Payload     []byte    `gorm:"type:jsonb;not null"`
	Attempts    int       `gorm:"not null"`
	AvailableAt time.Time `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

type reconciliationModel struct {
	ID                    string    `gorm:"type:uuid;primaryKey"`
	PaymentID             string    `gorm:"type:uuid;not null"`
	Provider              string    `gorm:"not null"`
	ProviderTransactionID string    `gorm:"not null"`
	LocalStatus           string    `gorm:"not null"`
	ProviderStatus        string    `gorm:"not null"`
	ExpectedAmountCents   int64     `gorm:"not null"`
	ProviderAmountCents   int64     `gorm:"not null"`
	Matched               bool      `gorm:"not null"`
	MismatchReason        string    `gorm:"not null"`
	CheckedAt             time.Time `gorm:"not null"`
}

type ledgerTransactionModel struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	Kind           string    `gorm:"not null"`
	ReferenceID    string    `gorm:"not null"`
	IdempotencyKey string    `gorm:"not null;uniqueIndex"`
	Description    string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

type ledgerEntryModel struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	TransactionID string    `gorm:"type:uuid;not null"`
	WalletID      string    `gorm:"type:uuid;not null"`
	AmountCents   int64     `gorm:"not null"`
	CreatedAt     time.Time `gorm:"not null"`
}

func (r *Repository) CreateWallet(ctx context.Context, wallet *domain.Wallet) (*domain.Wallet, error) {
	record := walletRecord(wallet)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("payments.wallets").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "owner_id"}},
			DoNothing: true,
		}).Create(&record).Error; err != nil {
			return err
		}

		// Query into a fresh model. Reusing record would make GORM include its
		// generated primary key in the lookup after ON CONFLICT DO NOTHING.
		var existing walletModel
		if err := tx.Table("payments.wallets").Where("owner_id = ?", wallet.OwnerID).First(&existing).Error; err != nil {
			return err
		}
		record = existing
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}
	return walletDomain(record), nil
}

func (r *Repository) FindWallet(ctx context.Context, ownerID string) (*domain.Wallet, error) {
	var record walletModel
	if err := r.db.WithContext(ctx).Table("payments.wallets").Where("owner_id = ?", ownerID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("find wallet: %w", err)
	}
	return walletDomain(record), nil
}

func (r *Repository) AdjustBalance(
	ctx context.Context,
	ownerID string,
	deltaCents int64,
	idempotencyKey, reason, fundingOwnerID string,
	now time.Time,
) (*domain.Wallet, error) {
	var result walletModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := findLedgerTransaction(tx, idempotencyKey)
		if err == nil {
			if existing.Kind != "balance_adjustment" || existing.ReferenceID != ownerID {
				return domain.ErrIdempotency
			}
			return tx.Table("payments.wallets").Where("owner_id = ?", ownerID).First(&result).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := ensureSystemWallet(tx, fundingOwnerID, now); err != nil {
			return err
		}
		wallets, err := lockWallets(tx, []string{ownerID, fundingOwnerID})
		if err != nil {
			return err
		}
		target, ok := wallets[ownerID]
		if !ok {
			return domain.ErrWalletNotFound
		}
		funding := wallets[fundingOwnerID]
		targetBalance, ok := safeAdd(target.BalanceCents, deltaCents)
		if !ok || (!target.AllowNegative && targetBalance < 0) {
			return domain.ErrInsufficientFunds
		}

		transactionID := uuid.NewString()
		transaction := ledgerTransactionModel{
			ID: transactionID, Kind: "balance_adjustment", ReferenceID: ownerID,
			IdempotencyKey: idempotencyKey, Description: reason, CreatedAt: now,
		}
		if err := tx.Table("payments.ledger_transactions").Create(&transaction).Error; err != nil {
			return err
		}
		entries := []ledgerEntryModel{
			newLedgerEntry(transactionID, target.ID, deltaCents, now),
			newLedgerEntry(transactionID, funding.ID, -deltaCents, now),
		}
		if err := tx.Table("payments.ledger_entries").Create(&entries).Error; err != nil {
			return err
		}
		if err := updateWalletBalance(tx, &target, deltaCents, now); err != nil {
			return err
		}
		if err := updateWalletBalance(tx, &funding, -deltaCents, now); err != nil {
			return err
		}
		result = target
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("adjust wallet balance: %w", err)
	}
	return walletDomain(result), nil
}

func (r *Repository) CreatePayment(
	ctx context.Context,
	payment *domain.Payment,
	allocations []domain.Allocation,
	platformOwnerID string,
	platformFeeBPS int32,
) (*domain.Payment, error) {
	var result paymentModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing paymentModel
		err := tx.Table("payments.payments").Where("order_id = ?", payment.OrderID).First(&existing).Error
		if err == nil {
			if existing.BuyerID != payment.BuyerID || existing.AmountCents != payment.AmountCents {
				return domain.ErrIdempotency
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		err = tx.Table("payments.payments").
			Where("buyer_id = ? AND idempotency_key = ?", payment.BuyerID, payment.IdempotencyKey).
			First(&existing).Error
		if err == nil {
			if existing.OrderID != payment.OrderID {
				return domain.ErrIdempotency
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		deltas := map[string]int64{payment.BuyerID: -payment.AmountCents}
		var totalFee int64
		for _, allocation := range allocations {
			fee := feeCents(allocation.AmountCents, platformFeeBPS)
			totalFee += fee
			deltas[allocation.SellerID] += allocation.AmountCents - fee
			deltas[platformOwnerID] += fee
		}
		if sumDeltas(deltas) != 0 {
			return domain.ErrInvalidInput
		}
		for ownerID := range deltas {
			if ownerID == payment.BuyerID {
				continue
			}
			if err := ensureWallet(tx, ownerID, payment.Currency, false, payment.CreatedAt); err != nil {
				return err
			}
		}
		wallets, err := lockWallets(tx, mapKeys(deltas))
		if err != nil {
			return err
		}
		if _, ok := wallets[payment.BuyerID]; !ok {
			return domain.ErrWalletNotFound
		}
		for ownerID, delta := range deltas {
			wallet, ok := wallets[ownerID]
			if !ok {
				return domain.ErrWalletNotFound
			}
			balance, ok := safeAdd(wallet.BalanceCents, delta)
			if !ok || (!wallet.AllowNegative && balance < 0) {
				return domain.ErrInsufficientFunds
			}
		}

		transactionID := uuid.NewString()
		transaction := ledgerTransactionModel{
			ID: transactionID, Kind: "payment", ReferenceID: payment.ID,
			IdempotencyKey: "payment:" + payment.IdempotencyKey,
			Description:    "payment for order " + payment.OrderID, CreatedAt: payment.CreatedAt,
		}
		if err := tx.Table("payments.ledger_transactions").Create(&transaction).Error; err != nil {
			return err
		}
		entries := make([]ledgerEntryModel, 0, len(deltas))
		for ownerID, delta := range deltas {
			if delta == 0 {
				continue
			}
			wallet := wallets[ownerID]
			entries = append(entries, newLedgerEntry(transactionID, wallet.ID, delta, payment.CreatedAt))
			if err := updateWalletBalance(tx, &wallet, delta, payment.CreatedAt); err != nil {
				return err
			}
		}
		if err := tx.Table("payments.ledger_entries").Create(&entries).Error; err != nil {
			return err
		}
		payment.PlatformFeeCents = totalFee
		result = paymentRecord(payment)
		return tx.Table("payments.payments").Create(&result).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return paymentDomain(result), nil
}

func (r *Repository) CreateGatewayPayment(
	ctx context.Context,
	payment *domain.Payment,
	allocations []domain.Allocation,
) (*domain.Payment, error) {
	var result paymentModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing paymentModel
		err := tx.Table("payments.payments").Where("order_id = ?", payment.OrderID).First(&existing).Error
		if err == nil {
			if existing.BuyerID != payment.BuyerID || existing.AmountCents != payment.AmountCents || existing.Provider != payment.Provider {
				return domain.ErrIdempotency
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		err = tx.Table("payments.payments").
			Where("buyer_id = ? AND idempotency_key = ?", payment.BuyerID, payment.IdempotencyKey).
			First(&existing).Error
		if err == nil {
			if existing.OrderID != payment.OrderID || existing.Provider != payment.Provider {
				return domain.ErrIdempotency
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		result = paymentRecord(payment)
		if err := tx.Table("payments.payments").Create(&result).Error; err != nil {
			return err
		}
		records := make([]paymentAllocationModel, 0, len(allocations))
		for _, allocation := range allocations {
			records = append(records, paymentAllocationModel{
				PaymentID: payment.ID, SellerID: allocation.SellerID,
				AmountCents: allocation.AmountCents, CreatedAt: payment.CreatedAt,
			})
		}
		return tx.Table("payments.payment_allocations").Create(&records).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create gateway payment: %w", err)
	}
	return paymentDomain(result), nil
}

func (r *Repository) ApplyGatewayResult(
	ctx context.Context,
	result domain.GatewayResult,
	platformOwnerID, clearingOwnerID string,
	platformFeeBPS int32,
	now time.Time,
) (*domain.Payment, bool, error) {
	var payment paymentModel
	var duplicate bool
	var processingErr error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		payload, marshalErr := json.Marshal(string(result.RawPayload))
		if marshalErr != nil {
			return marshalErr
		}
		webhook := webhookEventModel{
			ID: uuid.NewString(), Provider: result.Provider, EventID: result.EventID,
			ProviderReference: result.ProviderReference, SignatureValid: true,
			Payload: payload, ReceivedAt: now,
		}
		created := tx.Table("payments.webhook_events").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "provider"}, {Name: "event_id"}}, DoNothing: true,
		}).Create(&webhook)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			duplicate = true
			return tx.Table("payments.payments").
				Where("provider = ? AND provider_reference = ?", result.Provider, result.ProviderReference).
				First(&payment).Error
		}

		if err := tx.Table("payments.payments").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND provider_reference = ?", result.Provider, result.ProviderReference).
			First(&payment).Error; err != nil {
			processingErr = domain.ErrProviderReference
			return markWebhookProcessed(tx, webhook.ID, processingErr, now)
		}
		if payment.AmountCents != result.AmountCents {
			processingErr = domain.ErrAmountMismatch
			return markWebhookProcessed(tx, webhook.ID, processingErr, now)
		}
		if payment.Status == domain.StatusRefundPending {
			if result.Status != domain.StatusRefunded {
				return markWebhookProcessed(tx, webhook.ID, nil, now)
			}
			if err := reversePaymentLedger(
				tx, &payment, "gateway-refund:"+payment.ID,
				"gateway refund confirmed by reconciliation", apptrace.IDFromContext(ctx), now,
			); err != nil {
				return err
			}
			return markWebhookProcessed(tx, webhook.ID, nil, now)
		}
		if payment.Status != domain.StatusPending {
			duplicate = true
			return markWebhookProcessed(tx, webhook.ID, nil, now)
		}

		if result.Status == domain.StatusSucceeded {
			if err := r.postGatewayLedger(tx, &payment, platformOwnerID, clearingOwnerID, platformFeeBPS, now); err != nil {
				return err
			}
			payment.PaidAt = &now
		} else if result.Status != domain.StatusFailed {
			return markWebhookProcessed(tx, webhook.ID, nil, now)
		}

		payment.Status = result.Status
		payment.ProviderTransactionID = result.ProviderTransactionID
		payment.UpdatedAt = now
		updates := map[string]any{
			"status": payment.Status, "provider_transaction_id": payment.ProviderTransactionID,
			"platform_fee_cents": payment.PlatformFeeCents, "updated_at": now,
		}
		if payment.PaidAt != nil {
			updates["paid_at"] = *payment.PaidAt
		}
		if err := tx.Table("payments.payments").Where("id = ?", payment.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := createPaymentOutbox(tx, payment, apptrace.IDFromContext(ctx), now); err != nil {
			return err
		}
		return markWebhookProcessed(tx, webhook.ID, nil, now)
	})
	if err != nil {
		return nil, false, fmt.Errorf("apply gateway result: %w", err)
	}
	if processingErr != nil {
		return paymentDomain(payment), false, processingErr
	}
	return paymentDomain(payment), duplicate, nil
}

func (r *Repository) postGatewayLedger(
	tx *gorm.DB,
	payment *paymentModel,
	platformOwnerID, clearingOwnerID string,
	platformFeeBPS int32,
	now time.Time,
) error {
	var allocations []paymentAllocationModel
	if err := tx.Table("payments.payment_allocations").Where("payment_id = ?", payment.ID).Find(&allocations).Error; err != nil {
		return err
	}
	if len(allocations) == 0 {
		return domain.ErrInvalidInput
	}
	deltas := map[string]int64{clearingOwnerID: -payment.AmountCents}
	var totalFee int64
	for _, allocation := range allocations {
		fee := feeCents(allocation.AmountCents, platformFeeBPS)
		totalFee += fee
		deltas[allocation.SellerID] += allocation.AmountCents - fee
		deltas[platformOwnerID] += fee
	}
	if sumDeltas(deltas) != 0 {
		return domain.ErrInvalidInput
	}
	if err := ensureWallet(tx, clearingOwnerID, payment.Currency, true, now); err != nil {
		return err
	}
	for ownerID := range deltas {
		if ownerID == clearingOwnerID {
			continue
		}
		if err := ensureWallet(tx, ownerID, payment.Currency, false, now); err != nil {
			return err
		}
	}
	wallets, err := lockWallets(tx, mapKeys(deltas))
	if err != nil {
		return err
	}
	transactionID := uuid.NewString()
	transaction := ledgerTransactionModel{
		ID: transactionID, Kind: "gateway_payment", ReferenceID: payment.ID,
		IdempotencyKey: "gateway-payment:" + payment.ID,
		Description:    "gateway payment for order " + payment.OrderID, CreatedAt: now,
	}
	if err := tx.Table("payments.ledger_transactions").Create(&transaction).Error; err != nil {
		return err
	}
	entries := make([]ledgerEntryModel, 0, len(deltas))
	for ownerID, delta := range deltas {
		if delta == 0 {
			continue
		}
		wallet := wallets[ownerID]
		entries = append(entries, newLedgerEntry(transactionID, wallet.ID, delta, now))
		if err := updateWalletBalance(tx, &wallet, delta, now); err != nil {
			return err
		}
	}
	if err := tx.Table("payments.ledger_entries").Create(&entries).Error; err != nil {
		return err
	}
	payment.PlatformFeeCents = totalFee
	return nil
}

func (r *Repository) ListPendingGatewayPayments(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]*domain.Payment, error) {
	var records []paymentModel
	if err := r.db.WithContext(ctx).Table("payments.payments").
		Where("status IN ? AND provider <> ? AND updated_at <= ?",
			[]string{domain.StatusPending, domain.StatusRefundPending}, domain.ProviderWallet, before).
		Order("updated_at, id").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list pending gateway payments: %w", err)
	}
	payments := make([]*domain.Payment, 0, len(records))
	for _, record := range records {
		payments = append(payments, paymentDomain(record))
	}
	return payments, nil
}

func (r *Repository) RecordReconciliation(ctx context.Context, item domain.Reconciliation) error {
	record := reconciliationModel{
		ID: item.ID, PaymentID: item.PaymentID, Provider: item.Provider,
		ProviderTransactionID: item.ProviderTransactionID, LocalStatus: item.LocalStatus,
		ProviderStatus: item.ProviderStatus, ExpectedAmountCents: item.ExpectedAmountCents,
		ProviderAmountCents: item.ProviderAmountCents, Matched: item.Matched,
		MismatchReason: item.MismatchReason, CheckedAt: item.CheckedAt,
	}
	if err := r.db.WithContext(ctx).Table("payments.settlement_reconciliations").Create(&record).Error; err != nil {
		return fmt.Errorf("record settlement reconciliation: %w", err)
	}
	return nil
}

func (r *Repository) FindPayment(ctx context.Context, id, buyerID string) (*domain.Payment, error) {
	return findPayment(r.db.WithContext(ctx), "id = ? AND buyer_id = ?", id, buyerID)
}

func (r *Repository) FindPaymentInternal(ctx context.Context, id string) (*domain.Payment, error) {
	return findPayment(r.db.WithContext(ctx), "id = ?", id)
}

func (r *Repository) FindPaymentByOrder(ctx context.Context, orderID, buyerID string) (*domain.Payment, error) {
	return findPayment(r.db.WithContext(ctx), "order_id = ? AND buyer_id = ?", orderID, buyerID)
}

func (r *Repository) MarkRefundPending(
	ctx context.Context,
	paymentID string,
	now time.Time,
) (*domain.Payment, error) {
	var payment paymentModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("payments.payments").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", paymentID).First(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrPaymentNotFound
			}
			return err
		}
		if payment.Status == domain.StatusRefundPending || payment.Status == domain.StatusRefunded {
			return nil
		}
		if payment.Status != domain.StatusSucceeded {
			return domain.ErrPaymentState
		}
		payment.Status = domain.StatusRefundPending
		payment.UpdatedAt = now
		return tx.Table("payments.payments").Where("id = ?", payment.ID).
			Updates(map[string]any{"status": payment.Status, "updated_at": now}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("mark refund pending: %w", err)
	}
	return paymentDomain(payment), nil
}

func (r *Repository) RefundPayment(
	ctx context.Context,
	paymentID, idempotencyKey, reason string,
	now time.Time,
) (*domain.Payment, error) {
	var payment paymentModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("payments.payments").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", paymentID).First(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrPaymentNotFound
			}
			return err
		}
		return reversePaymentLedger(
			tx, &payment, idempotencyKey, reason, apptrace.IDFromContext(ctx), now,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("refund payment: %w", err)
	}
	return paymentDomain(payment), nil
}

func reversePaymentLedger(
	tx *gorm.DB,
	payment *paymentModel,
	idempotencyKey, reason, traceID string,
	now time.Time,
) error {
	if payment.Status == domain.StatusRefunded {
		return nil
	}
	if payment.Status != domain.StatusSucceeded && payment.Status != domain.StatusRefundPending {
		return domain.ErrPaymentState
	}
	existing, err := findLedgerTransaction(tx, idempotencyKey)
	if err == nil {
		if existing.Kind != "refund" || existing.ReferenceID != payment.ID {
			return domain.ErrIdempotency
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var original ledgerTransactionModel
	if err := tx.Table("payments.ledger_transactions").
		Where("kind IN ? AND reference_id = ?", []string{"payment", "gateway_payment"}, payment.ID).
		First(&original).Error; err != nil {
		return err
	}
	var originalEntries []ledgerEntryModel
	if err := tx.Table("payments.ledger_entries").
		Where("transaction_id = ?", original.ID).Find(&originalEntries).Error; err != nil {
		return err
	}
	walletIDs := make([]string, 0, len(originalEntries))
	for _, entry := range originalEntries {
		walletIDs = append(walletIDs, entry.WalletID)
	}
	wallets, err := lockWalletsByID(tx, walletIDs)
	if err != nil {
		return err
	}
	for _, entry := range originalEntries {
		wallet := wallets[entry.WalletID]
		delta := -entry.AmountCents
		balance, ok := safeAdd(wallet.BalanceCents, delta)
		if !ok || (!wallet.AllowNegative && balance < 0) {
			return domain.ErrInsufficientFunds
		}
	}

	refundID := uuid.NewString()
	transaction := ledgerTransactionModel{
		ID: refundID, Kind: "refund", ReferenceID: payment.ID,
		IdempotencyKey: idempotencyKey, Description: reason, CreatedAt: now,
	}
	if err := tx.Table("payments.ledger_transactions").Create(&transaction).Error; err != nil {
		return err
	}
	entries := make([]ledgerEntryModel, 0, len(originalEntries))
	for _, entry := range originalEntries {
		wallet := wallets[entry.WalletID]
		delta := -entry.AmountCents
		entries = append(entries, newLedgerEntry(refundID, wallet.ID, delta, now))
		if err := updateWalletBalance(tx, &wallet, delta, now); err != nil {
			return err
		}
	}
	if err := tx.Table("payments.ledger_entries").Create(&entries).Error; err != nil {
		return err
	}
	payment.Status = domain.StatusRefunded
	payment.UpdatedAt = now
	if err := tx.Table("payments.payments").Where("id = ?", payment.ID).
		Updates(map[string]any{"status": payment.Status, "updated_at": now}).Error; err != nil {
		return err
	}
	return createPaymentOutbox(tx, *payment, traceID, now)
}

func ensureSystemWallet(tx *gorm.DB, ownerID string, now time.Time) error {
	return ensureWallet(tx, ownerID, "VND", true, now)
}

func ensureWallet(tx *gorm.DB, ownerID, currency string, allowNegative bool, now time.Time) error {
	record := walletModel{
		ID: uuid.NewString(), OwnerID: ownerID, Currency: currency,
		AllowNegative: allowNegative, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Table("payments.wallets").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_id"}},
		DoNothing: true,
	}).Create(&record).Error
}

func lockWallets(tx *gorm.DB, ownerIDs []string) (map[string]walletModel, error) {
	sort.Strings(ownerIDs)
	var records []walletModel
	if err := tx.Table("payments.wallets").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_id IN ?", ownerIDs).Order("owner_id").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make(map[string]walletModel, len(records))
	for _, record := range records {
		result[record.OwnerID] = record
	}
	return result, nil
}

func lockWalletsByID(tx *gorm.DB, walletIDs []string) (map[string]walletModel, error) {
	sort.Strings(walletIDs)
	var records []walletModel
	if err := tx.Table("payments.wallets").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", walletIDs).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make(map[string]walletModel, len(records))
	for _, record := range records {
		result[record.ID] = record
	}
	return result, nil
}

func updateWalletBalance(tx *gorm.DB, wallet *walletModel, delta int64, now time.Time) error {
	balance, ok := safeAdd(wallet.BalanceCents, delta)
	if !ok {
		return domain.ErrInvalidInput
	}
	wallet.BalanceCents = balance
	wallet.UpdatedAt = now
	return tx.Table("payments.wallets").Where("id = ?", wallet.ID).
		Updates(map[string]any{"balance_cents": wallet.BalanceCents, "updated_at": now}).Error
}

func feeCents(amount int64, feeBPS int32) int64 {
	fee := int64(feeBPS)
	return amount/10000*fee + amount%10000*fee/10000
}

func safeAdd(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, false
	}
	if right < 0 && left < math.MinInt64-right {
		return 0, false
	}
	return left + right, true
}

func findLedgerTransaction(tx *gorm.DB, idempotencyKey string) (ledgerTransactionModel, error) {
	var record ledgerTransactionModel
	err := tx.Table("payments.ledger_transactions").Where("idempotency_key = ?", idempotencyKey).First(&record).Error
	return record, err
}

func findPayment(tx *gorm.DB, query string, arguments ...any) (*domain.Payment, error) {
	var record paymentModel
	if err := tx.Table("payments.payments").Where(query, arguments...).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("find payment: %w", err)
	}
	return paymentDomain(record), nil
}

func newLedgerEntry(transactionID, walletID string, amount int64, now time.Time) ledgerEntryModel {
	return ledgerEntryModel{
		ID: uuid.NewString(), TransactionID: transactionID, WalletID: walletID,
		AmountCents: amount, CreatedAt: now,
	}
}

func sumDeltas(deltas map[string]int64) int64 {
	var result int64
	for _, delta := range deltas {
		result += delta
	}
	return result
}

func mapKeys(values map[string]int64) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func markWebhookProcessed(tx *gorm.DB, id string, processingErr error, now time.Time) error {
	message := ""
	if processingErr != nil {
		message = processingErr.Error()
	}
	return tx.Table("payments.webhook_events").Where("id = ?", id).Updates(map[string]any{
		"processing_error": message, "processed_at": now,
	}).Error
}

func createPaymentOutbox(tx *gorm.DB, payment paymentModel, traceID string, now time.Time) error {
	eventType := "payment." + payment.Status
	payload, err := json.Marshal(map[string]any{
		"payment_id":              payment.ID,
		"order_id":                payment.OrderID,
		"buyer_id":                payment.BuyerID,
		"status":                  payment.Status,
		"provider":                payment.Provider,
		"provider_transaction_id": payment.ProviderTransactionID,
		"amount_cents":            payment.AmountCents,
		"currency":                payment.Currency,
		"occurred_at":             now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	record := paymentOutboxModel{
		ID: uuid.NewString(), AggregateID: payment.ID, EventType: eventType,
		TraceID: traceID, Payload: payload, AvailableAt: now, CreatedAt: now,
	}
	return tx.Table("payments.outbox_events").Create(&record).Error
}

func walletRecord(wallet *domain.Wallet) walletModel {
	return walletModel{
		ID: wallet.ID, OwnerID: wallet.OwnerID, BalanceCents: wallet.BalanceCents,
		Currency: wallet.Currency, AllowNegative: wallet.AllowNegative,
		CreatedAt: wallet.CreatedAt, UpdatedAt: wallet.UpdatedAt,
	}
}

func walletDomain(wallet walletModel) *domain.Wallet {
	return &domain.Wallet{
		ID: wallet.ID, OwnerID: wallet.OwnerID, BalanceCents: wallet.BalanceCents,
		Currency: wallet.Currency, AllowNegative: wallet.AllowNegative,
		CreatedAt: wallet.CreatedAt, UpdatedAt: wallet.UpdatedAt,
	}
}

func paymentRecord(payment *domain.Payment) paymentModel {
	var expiresAt, paidAt *time.Time
	if !payment.ExpiresAt.IsZero() {
		expiresAt = &payment.ExpiresAt
	}
	if !payment.PaidAt.IsZero() {
		paidAt = &payment.PaidAt
	}
	return paymentModel{
		ID: payment.ID, OrderID: payment.OrderID, BuyerID: payment.BuyerID,
		Status: payment.Status, AmountCents: payment.AmountCents,
		PlatformFeeCents: payment.PlatformFeeCents, Currency: payment.Currency,
		FailureReason: payment.FailureReason, IdempotencyKey: payment.IdempotencyKey,
		Provider: payment.Provider, ProviderReference: payment.ProviderReference,
		ProviderTransactionID: payment.ProviderTransactionID, CheckoutURL: payment.CheckoutURL,
		ExpiresAt: expiresAt, PaidAt: paidAt,
		CreatedAt: payment.CreatedAt, UpdatedAt: payment.UpdatedAt,
	}
}

func paymentDomain(payment paymentModel) *domain.Payment {
	result := &domain.Payment{
		ID: payment.ID, OrderID: payment.OrderID, BuyerID: payment.BuyerID,
		Status: payment.Status, AmountCents: payment.AmountCents,
		PlatformFeeCents: payment.PlatformFeeCents, Currency: payment.Currency,
		FailureReason: payment.FailureReason, IdempotencyKey: payment.IdempotencyKey,
		Provider: payment.Provider, ProviderReference: payment.ProviderReference,
		ProviderTransactionID: payment.ProviderTransactionID, CheckoutURL: payment.CheckoutURL,
		CreatedAt: payment.CreatedAt, UpdatedAt: payment.UpdatedAt,
	}
	if payment.ExpiresAt != nil {
		result.ExpiresAt = *payment.ExpiresAt
	}
	if payment.PaidAt != nil {
		result.PaidAt = *payment.PaidAt
	}
	return result
}
