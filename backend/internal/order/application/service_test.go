package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
)

type repositoryStub struct {
	cart       []*domain.CartItem
	order      *domain.Order
	clearCount int
	listCount  int
}

func (r *repositoryStub) AddCartItem(_ context.Context, item *domain.CartItem) (*domain.CartItem, error) {
	r.cart = append(r.cart, item)
	return item, nil
}
func (r *repositoryStub) UpdateCartItem(
	context.Context, string, string, int32, time.Time,
) (*domain.CartItem, error) {
	return nil, domain.ErrCartItemNotFound
}
func (r *repositoryStub) RemoveCartItems(context.Context, string, []string) error { return nil }
func (r *repositoryStub) ListCart(context.Context, string) ([]*domain.CartItem, error) {
	r.listCount++
	return r.cart, nil
}
func (r *repositoryStub) ClearCart(context.Context, string) error {
	r.clearCount++
	r.cart = nil
	return nil
}
func (r *repositoryStub) CreateOrder(_ context.Context, order *domain.Order) error {
	r.order = order
	return nil
}
func (r *repositoryStub) FindOrder(_ context.Context, userID, id string) (*domain.Order, error) {
	if r.order == nil || r.order.UserID != userID || r.order.ID != id {
		return nil, domain.ErrOrderNotFound
	}
	return r.order, nil
}
func (r *repositoryStub) FindOrderByIdempotency(
	_ context.Context, userID, key string,
) (*domain.Order, error) {
	if r.order == nil || r.order.UserID != userID || r.order.IdempotencyKey != key {
		return nil, domain.ErrOrderNotFound
	}
	return r.order, nil
}
func (r *repositoryStub) ListOrders(
	context.Context, string, int32, *domain.Cursor,
) ([]*domain.Order, error) {
	return nil, nil
}
func (r *repositoryStub) UpdateOrderState(
	_ context.Context,
	_, _ string,
	_ []string,
	status, paymentID, failureReason string,
	now time.Time,
) (*domain.Order, error) {
	r.order.Status = status
	if paymentID != "" {
		r.order.PaymentID = paymentID
	}
	r.order.FailureReason = failureReason
	r.order.UpdatedAt = now
	return r.order, nil
}
func (r *repositoryStub) ListOrdersForReconciliation(
	context.Context, time.Time, time.Time, int,
) ([]*domain.Order, error) {
	if r.order == nil {
		return nil, nil
	}
	return []*domain.Order{r.order}, nil
}

type bookClientStub struct {
	book         domain.BookSnapshot
	reserveCount int
	commitCount  int
	releaseCount int
}

func (b *bookClientStub) GetBook(context.Context, string) (domain.BookSnapshot, error) {
	return b.book, nil
}
func (b *bookClientStub) ReserveStock(context.Context, string, string, int32, string) error {
	b.reserveCount++
	return nil
}
func (b *bookClientStub) CommitStock(context.Context, string, string) error {
	b.commitCount++
	return nil
}
func (b *bookClientStub) ReleaseStock(context.Context, string, string) error {
	b.releaseCount++
	return nil
}

type paymentClientStub struct {
	payment       *domain.Payment
	refundPayment *domain.Payment
	createErr     error
	createCall    int
	refundCall    int
}

type cartCacheStub struct {
	mu       sync.Mutex
	values   map[string][]byte
	versions map[string]int64
}

func newCartCacheStub() *cartCacheStub {
	return &cartCacheStub{values: make(map[string][]byte), versions: make(map[string]int64)}
}

func (c *cartCacheStub) GetJSON(_ context.Context, key string, destination any) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, ok := c.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(payload, destination)
}

func (c *cartCacheStub) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = payload
	return nil
}

func (c *cartCacheStub) Version(_ context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.versions[key], nil
}

func (c *cartCacheStub) BumpVersion(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.versions[key]++
	return nil
}

func (c *cartCacheStub) TryLock(context.Context, string, time.Duration) (string, bool, error) {
	return "test-lock", true, nil
}

func (c *cartCacheStub) Unlock(context.Context, string, string) error { return nil }

func (p *paymentClientStub) CreatePayment(
	context.Context, string, string, int64, []domain.PaymentAllocation, string, domain.PaymentOptions,
) (*domain.Payment, error) {
	p.createCall++
	return p.payment, p.createErr
}
func (p *paymentClientStub) GetPaymentByOrder(context.Context, string, string) (*domain.Payment, error) {
	if p.payment == nil {
		return nil, domain.ErrPaymentNotFound
	}
	return p.payment, nil
}
func (p *paymentClientStub) RefundPayment(
	context.Context, string, string, string,
) (*domain.Payment, error) {
	p.refundCall++
	if p.refundPayment != nil {
		return p.refundPayment, nil
	}
	if p.payment != nil {
		result := *p.payment
		result.Status = "refunded"
		return &result, nil
	}
	return p.payment, nil
}

func newCheckoutFixture() (*Service, *repositoryStub, *bookClientStub, *paymentClientStub, string) {
	userID := uuid.NewString()
	bookID := uuid.NewString()
	repository := &repositoryStub{cart: []*domain.CartItem{{
		ID: uuid.NewString(), UserID: userID, BookID: bookID, Quantity: 2,
	}}}
	books := &bookClientStub{book: domain.BookSnapshot{
		ID: bookID, SellerID: uuid.NewString(), Title: "Clean Architecture", PriceCents: 5000,
	}}
	payments := &paymentClientStub{}
	service := NewService(repository, books, payments, Config{Currency: "VND", PlatformOwnerID: "platform"})
	return service, repository, books, payments, userID
}

func TestCreateOrderReservesStockAndIsIdempotent(t *testing.T) {
	service, repository, books, _, userID := newCheckoutFixture()

	first, err := service.CreateOrder(context.Background(), userID, "order-attempt-1")
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	second, err := service.CreateOrder(context.Background(), userID, "order-attempt-1")
	if err != nil {
		t.Fatalf("CreateOrder() retry error = %v", err)
	}
	if first.ID != second.ID || first.Status != domain.StatusStockReserved {
		t.Fatalf("idempotent orders differ: first=%+v second=%+v", first, second)
	}
	if books.reserveCount != 1 {
		t.Fatalf("ReserveStock calls = %d, want 1", books.reserveCount)
	}
	if repository.clearCount != 1 || len(repository.cart) != 0 {
		t.Fatalf("cart was not cleared after reservation")
	}
}

func TestPayOrderCommitsStock(t *testing.T) {
	service, repository, books, payments, userID := newCheckoutFixture()
	order, err := service.CreateOrder(context.Background(), userID, "order-success")
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	payments.payment = &domain.Payment{
		ID: uuid.NewString(), OrderID: order.ID, BuyerID: userID, Status: "succeeded", AmountCents: 10000,
	}

	payment, err := service.PayOrder(context.Background(), userID, order.ID, "payment-success", domain.PaymentOptions{})
	if err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if payment.ID != payments.payment.ID || repository.order.Status != domain.StatusConfirmed {
		t.Fatalf("payment/order not confirmed: payment=%+v order=%+v", payment, repository.order)
	}
	if books.commitCount != 1 || books.releaseCount != 0 || payments.createCall != 1 {
		t.Fatalf("unexpected effects: commits=%d releases=%d payments=%d", books.commitCount, books.releaseCount, payments.createCall)
	}
}

func TestPayOrderDeclineReleasesStock(t *testing.T) {
	service, repository, books, payments, userID := newCheckoutFixture()
	order, err := service.CreateOrder(context.Background(), userID, "order-declined")
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	payments.createErr = domain.ErrPaymentDeclined

	_, err = service.PayOrder(context.Background(), userID, order.ID, "payment-declined", domain.PaymentOptions{})
	if !errors.Is(err, domain.ErrPaymentDeclined) {
		t.Fatalf("PayOrder() error = %v, want payment declined", err)
	}
	if books.releaseCount != 1 || books.commitCount != 0 {
		t.Fatalf("unexpected stock effects: releases=%d commits=%d", books.releaseCount, books.commitCount)
	}
	if repository.order.Status != domain.StatusCancelled {
		t.Fatalf("order status = %q, want cancelled", repository.order.Status)
	}
}

func TestRefundEventCompletesPendingCompensation(t *testing.T) {
	service, repository, books, payments, userID := newCheckoutFixture()
	paymentID := uuid.NewString()
	repository.order = &domain.Order{
		ID: uuid.NewString(), UserID: userID, Status: domain.StatusCompensationPending,
		PaymentID: paymentID, Items: []domain.Item{{BookID: uuid.NewString()}},
	}
	payments.refundPayment = &domain.Payment{ID: paymentID, Status: "refunded"}

	if err := service.HandlePaymentEvent(context.Background(), repository.order.ID, userID); err != nil {
		t.Fatalf("HandlePaymentEvent() error = %v", err)
	}
	if repository.order.Status != domain.StatusCancelled || payments.refundCall != 1 || books.releaseCount != 1 {
		t.Fatalf(
			"unexpected compensation effects: status=%s refunds=%d releases=%d",
			repository.order.Status, payments.refundCall, books.releaseCount,
		)
	}
}

func TestListCartUsesCacheAndMutationInvalidatesVersion(t *testing.T) {
	service, repository, _, _, userID := newCheckoutFixture()
	service.SetCache(newCartCacheStub(), time.Minute, time.Second)

	for range 2 {
		cart, err := service.ListCart(context.Background(), userID)
		if err != nil || len(cart) != 1 {
			t.Fatalf("ListCart() cart = %+v, error = %v", cart, err)
		}
	}
	if repository.listCount != 1 {
		t.Fatalf("repository ListCart calls = %d, want 1", repository.listCount)
	}

	if _, err := service.AddToCart(context.Background(), userID, uuid.NewString(), 1); err != nil {
		t.Fatalf("AddToCart() error = %v", err)
	}
	if _, err := service.ListCart(context.Background(), userID); err != nil {
		t.Fatalf("ListCart() after invalidation error = %v", err)
	}
	if repository.listCount != 2 {
		t.Fatalf("repository ListCart calls after invalidation = %d, want 2", repository.listCount)
	}
}
