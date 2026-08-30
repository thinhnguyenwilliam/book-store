package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"golang.org/x/sync/singleflight"
)

type Config struct {
	Currency              string
	PlatformOwnerID       string
	StockReservationTTL   time.Duration
	PaymentReconcileGrace time.Duration
}

type Service struct {
	repository  Repository
	books       BookClient
	payments    PaymentClient
	cache       Cache
	config      Config
	now         func() time.Time
	cacheTTL    time.Duration
	lockTTL     time.Duration
	cacheFlight singleflight.Group
}

func NewService(repository Repository, books BookClient, payments PaymentClient, config Config) *Service {
	config.Currency = strings.ToUpper(strings.TrimSpace(config.Currency))
	if config.Currency == "" {
		config.Currency = "VND"
	}
	if config.PlatformOwnerID == "" {
		config.PlatformOwnerID = "platform"
	}
	if config.StockReservationTTL <= 0 {
		config.StockReservationTTL = 15 * time.Minute
	}
	if config.PaymentReconcileGrace <= 0 {
		config.PaymentReconcileGrace = 30 * time.Second
	}
	return &Service{repository: repository, books: books, payments: payments, config: config, now: time.Now}
}

func (s *Service) SetCache(cache Cache, ttl, lockTTL time.Duration) {
	if cache == nil || ttl <= 0 || lockTTL <= 0 {
		return
	}
	s.cache = cache
	s.cacheTTL = ttl
	s.lockTTL = lockTTL
}

func (s *Service) AddToCart(ctx context.Context, userID, bookID string, quantity int32) (*domain.CartItem, error) {
	if !validUUID(userID) || !validUUID(bookID) || quantity < 1 || quantity > 100 {
		return nil, domain.ErrInvalidInput
	}
	if _, err := s.books.GetBook(ctx, bookID); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	item, err := s.repository.AddCartItem(ctx, &domain.CartItem{
		ID: uuid.NewString(), UserID: userID, BookID: bookID, Quantity: quantity,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	s.invalidateCart(ctx, userID)
	return item, nil
}

func (s *Service) Reconcile(ctx context.Context, limit int) error {
	now := s.now().UTC()
	orders, err := s.repository.ListOrdersForReconciliation(
		ctx, now, now.Add(-s.config.PaymentReconcileGrace), limit,
	)
	if err != nil {
		return err
	}
	var result error
	for _, order := range orders {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		switch order.Status {
		case domain.StatusPending:
			s.releaseItems(ctx, order.ID, order.Items)
			_, reconcileErr := s.repository.UpdateOrderState(
				ctx, order.UserID, order.ID, []string{domain.StatusPending},
				domain.StatusCancelled, "", "stock reservation expired", s.now().UTC(),
			)
			result = errors.Join(result, reconcileErr)
		case domain.StatusStockReserved:
			if releaseErr := s.releaseItemsStrict(ctx, order.ID, order.Items); releaseErr != nil {
				result = errors.Join(result, releaseErr)
				continue
			}
			_, reconcileErr := s.repository.UpdateOrderState(
				ctx, order.UserID, order.ID, []string{domain.StatusStockReserved},
				domain.StatusCancelled, "", "stock reservation expired", s.now().UTC(),
			)
			result = errors.Join(result, reconcileErr)
		case domain.StatusPaymentPending:
			result = errors.Join(result, s.reconcilePaymentPending(ctx, order))
		case domain.StatusCompensationPending:
			result = errors.Join(result, s.reconcileCompensation(ctx, order))
		}
	}
	return result
}

func (s *Service) reconcilePaymentPending(ctx context.Context, order *domain.Order) error {
	payment, err := s.payments.GetPaymentByOrder(ctx, order.ID, order.UserID)
	if errors.Is(err, domain.ErrPaymentNotFound) {
		if releaseErr := s.releaseItemsStrict(ctx, order.ID, order.Items); releaseErr != nil {
			return releaseErr
		}
		_, stateErr := s.repository.UpdateOrderState(
			ctx, order.UserID, order.ID, []string{domain.StatusPaymentPending},
			domain.StatusCancelled, "", "stock reservation expired", s.now().UTC(),
		)
		return stateErr
	}
	if err != nil {
		return err
	}
	if payment.Status == "pending" {
		return nil
	}
	if payment.Status == "failed" {
		if releaseErr := s.releaseItemsStrict(ctx, order.ID, order.Items); releaseErr != nil {
			return releaseErr
		}
		_, stateErr := s.repository.UpdateOrderState(
			ctx, order.UserID, order.ID, []string{domain.StatusPaymentPending},
			domain.StatusCancelled, payment.ID, "payment failed", s.now().UTC(),
		)
		return stateErr
	}
	if payment.Status != "succeeded" {
		return domain.ErrOrderState
	}
	for _, item := range order.Items {
		if commitErr := s.books.CommitStock(ctx, order.ID, item.BookID); commitErr != nil {
			return s.compensatePaidOrder(ctx, order, payment, commitErr)
		}
	}
	_, err = s.repository.UpdateOrderState(
		ctx, order.UserID, order.ID, []string{domain.StatusPaymentPending},
		domain.StatusConfirmed, payment.ID, "", s.now().UTC(),
	)
	return err
}

func (s *Service) HandlePaymentEvent(ctx context.Context, orderID, buyerID string) error {
	if !validUUID(orderID) || !validUUID(buyerID) {
		return domain.ErrInvalidInput
	}
	order, err := s.repository.FindOrder(ctx, buyerID, orderID)
	if err != nil {
		return err
	}
	if order.Status == domain.StatusConfirmed || order.Status == domain.StatusCancelled {
		return nil
	}
	if order.Status == domain.StatusCompensationPending {
		return s.reconcileCompensation(ctx, order)
	}
	if order.Status != domain.StatusPaymentPending {
		return domain.ErrOrderState
	}
	return s.reconcilePaymentPending(ctx, order)
}

func (s *Service) reconcileCompensation(ctx context.Context, order *domain.Order) error {
	if order.PaymentID != "" {
		payment, err := s.payments.RefundPayment(
			ctx, order.PaymentID, "refund:"+order.PaymentID, "order compensation retry",
		)
		if err != nil {
			return err
		}
		if payment.Status != "refunded" {
			return domain.ErrRefundPending
		}
	}
	if err := s.releaseItemsStrict(ctx, order.ID, order.Items); err != nil {
		return err
	}
	_, err := s.repository.UpdateOrderState(
		ctx, order.UserID, order.ID, []string{domain.StatusCompensationPending},
		domain.StatusCancelled, order.PaymentID, order.FailureReason, s.now().UTC(),
	)
	return err
}

func (s *Service) UpdateCartItem(
	ctx context.Context,
	userID, itemID string,
	quantity int32,
) (*domain.CartItem, error) {
	if !validUUID(userID) || !validUUID(itemID) || quantity < 1 || quantity > 100 {
		return nil, domain.ErrInvalidInput
	}
	item, err := s.repository.UpdateCartItem(ctx, userID, itemID, quantity, s.now().UTC())
	if err != nil {
		return nil, err
	}
	s.invalidateCart(ctx, userID)
	return item, nil
}

func (s *Service) RemoveFromCart(ctx context.Context, userID string, itemIDs []string) error {
	if !validUUID(userID) || len(itemIDs) == 0 {
		return domain.ErrInvalidInput
	}
	for _, itemID := range itemIDs {
		if !validUUID(itemID) {
			return domain.ErrInvalidInput
		}
	}
	if err := s.repository.RemoveCartItems(ctx, userID, itemIDs); err != nil {
		return err
	}
	s.invalidateCart(ctx, userID)
	return nil
}

func (s *Service) ListCart(ctx context.Context, userID string) ([]*domain.CartItem, error) {
	if !validUUID(userID) {
		return nil, domain.ErrInvalidInput
	}
	return s.listCartCached(ctx, userID)
}

func (s *Service) CreateOrder(ctx context.Context, userID, idempotencyKey string) (*domain.Order, error) {
	if !validUUID(userID) || strings.TrimSpace(idempotencyKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	existing, err := s.repository.FindOrderByIdempotency(ctx, userID, idempotencyKey)
	if err == nil {
		if existing.Status == domain.StatusPending {
			return s.reserveOrder(ctx, existing)
		}
		return existing, nil
	}
	if !errors.Is(err, domain.ErrOrderNotFound) {
		return nil, err
	}

	cart, err := s.repository.ListCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(cart) == 0 {
		return nil, domain.ErrCartEmpty
	}
	now := s.now().UTC()
	order := &domain.Order{
		ID: uuid.NewString(), UserID: userID, Status: domain.StatusPending,
		Currency: s.config.Currency, IdempotencyKey: idempotencyKey,
		CreatedAt: now, UpdatedAt: now,
		ReservationExpiresAt: now.Add(s.config.StockReservationTTL),
		Items:                make([]domain.Item, 0, len(cart)),
	}
	for _, cartItem := range cart {
		book, getErr := s.books.GetBook(ctx, cartItem.BookID)
		if getErr != nil {
			return nil, getErr
		}
		sellerID := book.SellerID
		if sellerID == "" {
			sellerID = s.config.PlatformOwnerID
		}
		if book.PriceCents < 0 || book.PriceCents > math.MaxInt64/int64(cartItem.Quantity) {
			return nil, domain.ErrInvalidInput
		}
		subtotal := book.PriceCents * int64(cartItem.Quantity)
		if order.TotalCents > math.MaxInt64-subtotal {
			return nil, domain.ErrInvalidInput
		}
		order.Items = append(order.Items, domain.Item{
			ID: uuid.NewString(), BookID: book.ID, SellerID: sellerID, Title: book.Title,
			UnitPriceCents: book.PriceCents, Quantity: cartItem.Quantity, SubtotalCents: subtotal,
		})
		order.TotalCents += subtotal
	}
	if order.TotalCents <= 0 {
		return nil, domain.ErrInvalidInput
	}
	if err := s.repository.CreateOrder(ctx, order); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			existing, findErr := s.repository.FindOrderByIdempotency(ctx, userID, idempotencyKey)
			if findErr != nil {
				return nil, findErr
			}
			if existing.Status == domain.StatusPending {
				return s.reserveOrder(ctx, existing)
			}
			return existing, nil
		}
		return nil, err
	}
	return s.reserveOrder(ctx, order)
}

func (s *Service) reserveOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	reserved := make([]domain.Item, 0, len(order.Items))
	for _, item := range order.Items {
		err := s.books.ReserveStock(
			ctx, order.ID, item.BookID, item.Quantity, "stock:"+order.ID+":"+item.BookID,
		)
		if err != nil {
			s.releaseItems(ctx, order.ID, append(reserved, item))
			_, _ = s.repository.UpdateOrderState(
				ctx, order.UserID, order.ID, []string{domain.StatusPending},
				domain.StatusCancelled, "", safeReason(err), s.now().UTC(),
			)
			return nil, fmt.Errorf("reserve order stock: %w", err)
		}
		reserved = append(reserved, item)
	}
	updated, err := s.repository.UpdateOrderState(
		ctx, order.UserID, order.ID, []string{domain.StatusPending},
		domain.StatusStockReserved, "", "", s.now().UTC(),
	)
	if err != nil {
		s.releaseItems(ctx, order.ID, reserved)
		return nil, err
	}
	if err := s.repository.ClearCart(ctx, order.UserID); err != nil {
		// Cart cleanup is retryable and must not invalidate an otherwise valid order.
		return updated, nil //nolint:nilerr // Stock is reserved and the order is already durable.
	}
	s.invalidateCart(ctx, order.UserID)
	return updated, nil
}

func (s *Service) GetOrder(ctx context.Context, userID, id string) (*domain.Order, error) {
	if !validUUID(userID) || !validUUID(id) {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindOrder(ctx, userID, id)
}

func (s *Service) ListOrders(ctx context.Context, userID, rawCursor string, limit int32) (domain.Page, error) {
	if !validUUID(userID) {
		return domain.Page{}, domain.ErrInvalidInput
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return domain.Page{}, err
	}
	orders, err := s.repository.ListOrders(ctx, userID, limit+1, cursor)
	if err != nil {
		return domain.Page{}, err
	}
	hasMore := len(orders) > int(limit)
	if hasMore {
		orders = orders[:limit]
	}
	page := domain.Page{Orders: orders, HasMore: hasMore}
	if hasMore && len(orders) > 0 {
		last := orders[len(orders)-1]
		page.NextCursor, err = encodeCursor(domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, err
}

func (s *Service) CancelOrder(ctx context.Context, userID, id string) (*domain.Order, error) {
	order, err := s.GetOrder(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if order.Status == domain.StatusCancelled {
		return order, nil
	}
	if order.Status != domain.StatusPending && order.Status != domain.StatusStockReserved {
		return nil, domain.ErrOrderState
	}
	if order.Status == domain.StatusStockReserved {
		if err := s.releaseItemsStrict(ctx, order.ID, order.Items); err != nil {
			return nil, err
		}
	}
	return s.repository.UpdateOrderState(
		ctx, userID, id, []string{domain.StatusPending, domain.StatusStockReserved},
		domain.StatusCancelled, "", "cancelled by customer", s.now().UTC(),
	)
}

func (s *Service) PayOrder(
	ctx context.Context,
	userID, orderID, idempotencyKey string,
	options domain.PaymentOptions,
) (*domain.Payment, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, domain.ErrInvalidInput
	}
	order, err := s.GetOrder(ctx, userID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status == domain.StatusConfirmed && order.PaymentID != "" {
		return s.payments.GetPaymentByOrder(ctx, order.ID, userID)
	}
	if order.Status != domain.StatusStockReserved && order.Status != domain.StatusPaymentPending {
		return nil, domain.ErrOrderState
	}
	if order.Status == domain.StatusStockReserved {
		updated, stateErr := s.repository.UpdateOrderState(
			ctx, userID, order.ID, []string{domain.StatusStockReserved},
			domain.StatusPaymentPending, "", "", s.now().UTC(),
		)
		if stateErr != nil {
			return nil, stateErr
		}
		order = updated
	}
	allocationsBySeller := make(map[string]int64)
	for _, item := range order.Items {
		allocationsBySeller[item.SellerID] += item.SubtotalCents
	}
	allocations := make([]domain.PaymentAllocation, 0, len(allocationsBySeller))
	for sellerID, amount := range allocationsBySeller {
		if amount == 0 {
			continue
		}
		allocations = append(allocations, domain.PaymentAllocation{SellerID: sellerID, AmountCents: amount})
	}
	payment, err := s.payments.CreatePayment(
		ctx, order.ID, userID, order.TotalCents, allocations, idempotencyKey, options,
	)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentDeclined) {
			if releaseErr := s.releaseItemsStrict(ctx, order.ID, order.Items); releaseErr != nil {
				return nil, errors.Join(err, releaseErr)
			}
			_, _ = s.repository.UpdateOrderState(
				ctx, userID, order.ID, []string{domain.StatusPaymentPending},
				domain.StatusCancelled, "", "payment declined", s.now().UTC(),
			)
			return nil, err
		}
		// The remote result may be unknown after a timeout. Reconcile by order ID
		// before performing a compensation that could release paid inventory.
		payment, err = s.payments.GetPaymentByOrder(ctx, order.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("payment result is unknown; retry with the same idempotency key: %w", err)
		}
	}
	if payment.Status == "pending" {
		return payment, nil
	}
	if payment.Status != "succeeded" {
		return nil, domain.ErrPaymentDeclined
	}
	for _, item := range order.Items {
		if err := s.books.CommitStock(ctx, order.ID, item.BookID); err != nil {
			return nil, s.compensatePaidOrder(ctx, order, payment, err)
		}
	}
	if _, err := s.repository.UpdateOrderState(
		ctx, userID, order.ID, []string{domain.StatusPaymentPending},
		domain.StatusConfirmed, payment.ID, "", s.now().UTC(),
	); err != nil {
		return nil, s.compensatePaidOrder(ctx, order, payment, err)
	}
	return payment, nil
}

func (s *Service) compensatePaidOrder(
	ctx context.Context,
	order *domain.Order,
	payment *domain.Payment,
	cause error,
) error {
	refundKey := "refund:" + payment.ID
	refundedPayment, refundErr := s.payments.RefundPayment(ctx, payment.ID, refundKey, "order compensation")
	if refundErr == nil && refundedPayment.Status != "refunded" {
		refundErr = domain.ErrRefundPending
	}
	releaseErr := s.releaseItemsStrict(ctx, order.ID, order.Items)
	status := domain.StatusCancelled
	if refundErr != nil || releaseErr != nil {
		status = domain.StatusCompensationPending
	}
	_, stateErr := s.repository.UpdateOrderState(
		ctx, order.UserID, order.ID, []string{domain.StatusPaymentPending},
		status, payment.ID, safeReason(cause), s.now().UTC(),
	)
	return errors.Join(cause, refundErr, releaseErr, stateErr)
}

func (s *Service) releaseItems(ctx context.Context, orderID string, items []domain.Item) {
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		_ = s.books.ReleaseStock(ctx, orderID, item.BookID)
	}
}

func (s *Service) releaseItemsStrict(ctx context.Context, orderID string, items []domain.Item) error {
	var result error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		result = errors.Join(result, s.books.ReleaseStock(ctx, orderID, item.BookID))
	}
	return result
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, domain.ErrPaymentDeclined) {
		return "payment declined"
	}
	return "checkout operation failed"
}
