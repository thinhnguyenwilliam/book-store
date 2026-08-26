package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type cartItemModel struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	UserID    string    `gorm:"type:uuid;not null"`
	BookID    string    `gorm:"type:uuid;not null"`
	Quantity  int32     `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type orderModel struct {
	ID                   string    `gorm:"type:uuid;primaryKey"`
	UserID               string    `gorm:"type:uuid;not null"`
	Status               string    `gorm:"not null"`
	TotalCents           int64     `gorm:"not null"`
	Currency             string    `gorm:"not null"`
	PaymentID            *string   `gorm:"type:uuid"`
	FailureReason        string    `gorm:"not null"`
	IdempotencyKey       string    `gorm:"not null"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
	ReservationExpiresAt time.Time `gorm:"not null"`
}

type orderItemModel struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	OrderID        string    `gorm:"type:uuid;not null"`
	BookID         string    `gorm:"type:uuid;not null"`
	SellerID       string    `gorm:"not null"`
	Title          string    `gorm:"not null"`
	UnitPriceCents int64     `gorm:"not null"`
	Quantity       int32     `gorm:"not null"`
	SubtotalCents  int64     `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (r *Repository) AddCartItem(ctx context.Context, item *domain.CartItem) (*domain.CartItem, error) {
	var result cartItemModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Table("orders.cart_items").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND book_id = ?", item.UserID, item.BookID).First(&result).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = cartItemRecord(item)
			return tx.Table("orders.cart_items").Create(&result).Error
		}
		if err != nil {
			return err
		}
		quantity := result.Quantity + item.Quantity
		if quantity > 100 {
			return domain.ErrInvalidInput
		}
		result.Quantity = quantity
		result.UpdatedAt = item.UpdatedAt
		return tx.Table("orders.cart_items").Where("id = ?", result.ID).
			Updates(map[string]any{"quantity": quantity, "updated_at": item.UpdatedAt}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("add cart item: %w", err)
	}
	return cartItemDomain(result), nil
}

func (r *Repository) UpdateCartItem(
	ctx context.Context,
	userID, itemID string,
	quantity int32,
	now time.Time,
) (*domain.CartItem, error) {
	result := r.db.WithContext(ctx).Table("orders.cart_items").
		Where("id = ? AND user_id = ?", itemID, userID).
		Updates(map[string]any{"quantity": quantity, "updated_at": now})
	if result.Error != nil {
		return nil, fmt.Errorf("update cart item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrCartItemNotFound
	}
	var record cartItemModel
	if err := r.db.WithContext(ctx).Table("orders.cart_items").Where("id = ?", itemID).First(&record).Error; err != nil {
		return nil, fmt.Errorf("reload cart item: %w", err)
	}
	return cartItemDomain(record), nil
}

func (r *Repository) RemoveCartItems(ctx context.Context, userID string, itemIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("orders.cart_items").Where("user_id = ? AND id IN ?", userID, itemIDs).
			Delete(&cartItemModel{})
		if result.Error != nil {
			return fmt.Errorf("remove cart items: %w", result.Error)
		}
		if result.RowsAffected != int64(len(itemIDs)) {
			return domain.ErrCartItemNotFound
		}
		return nil
	})
}

func (r *Repository) ListCart(ctx context.Context, userID string) ([]*domain.CartItem, error) {
	var records []cartItemModel
	if err := r.db.WithContext(ctx).Table("orders.cart_items").
		Where("user_id = ?", userID).Order("created_at, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list cart: %w", err)
	}
	result := make([]*domain.CartItem, 0, len(records))
	for _, record := range records {
		result = append(result, cartItemDomain(record))
	}
	return result, nil
}

func (r *Repository) ClearCart(ctx context.Context, userID string) error {
	if err := r.db.WithContext(ctx).Table("orders.cart_items").Where("user_id = ?", userID).
		Delete(&cartItemModel{}).Error; err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}
	return nil
}

func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := orderRecord(order)
		if err := tx.Table("orders.orders").Create(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrIdempotencyConflict
			}
			return err
		}
		items := make([]orderItemModel, 0, len(order.Items))
		for _, item := range order.Items {
			items = append(items, orderItemRecord(order.ID, item, order.CreatedAt))
		}
		return tx.Table("orders.order_items").Create(&items).Error
	})
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (r *Repository) FindOrder(ctx context.Context, userID, id string) (*domain.Order, error) {
	return r.findOrder(r.db.WithContext(ctx), "user_id = ? AND id = ?", userID, id)
}

func (r *Repository) FindOrderByIdempotency(
	ctx context.Context,
	userID, idempotencyKey string,
) (*domain.Order, error) {
	return r.findOrder(
		r.db.WithContext(ctx), "user_id = ? AND idempotency_key = ?", userID, idempotencyKey,
	)
}

func (r *Repository) ListOrders(
	ctx context.Context,
	userID string,
	limit int32,
	cursor *domain.Cursor,
) ([]*domain.Order, error) {
	query := r.db.WithContext(ctx).Table("orders.orders").Where("user_id = ?", userID).
		Order("created_at DESC").Order("id DESC").Limit(int(limit))
	if cursor != nil {
		query = query.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []orderModel
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	if len(records) == 0 {
		return []*domain.Order{}, nil
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	var itemRecords []orderItemModel
	if err := r.db.WithContext(ctx).Table("orders.order_items").Where("order_id IN ?", ids).
		Order("created_at, id").Find(&itemRecords).Error; err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	itemsByOrder := make(map[string][]domain.Item, len(records))
	for _, record := range itemRecords {
		itemsByOrder[record.OrderID] = append(itemsByOrder[record.OrderID], orderItemDomain(record))
	}
	result := make([]*domain.Order, 0, len(records))
	for _, record := range records {
		order := orderDomain(record)
		order.Items = itemsByOrder[order.ID]
		result = append(result, order)
	}
	return result, nil
}

func (r *Repository) UpdateOrderState(
	ctx context.Context,
	userID, id string,
	allowedStatuses []string,
	status, paymentID, failureReason string,
	now time.Time,
) (*domain.Order, error) {
	updates := map[string]any{
		"status": status, "failure_reason": failureReason, "updated_at": now,
	}
	if paymentID != "" {
		updates["payment_id"] = paymentID
	}
	result := r.db.WithContext(ctx).Table("orders.orders").
		Where("user_id = ? AND id = ? AND status IN ?", userID, id, allowedStatuses).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update order state: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		current, err := r.FindOrder(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		if current.Status == status {
			return current, nil
		}
		return nil, domain.ErrOrderState
	}
	return r.FindOrder(ctx, userID, id)
}

func (r *Repository) ListOrdersForReconciliation(
	ctx context.Context,
	now, paymentCutoff time.Time,
	limit int,
) ([]*domain.Order, error) {
	var records []orderModel
	if err := r.db.WithContext(ctx).Table("orders.orders").
		Where(
			"status = ? OR (status IN ? AND reservation_expires_at <= ?) OR (status = ? AND updated_at <= ?)",
			domain.StatusCompensationPending,
			[]string{domain.StatusPending, domain.StatusStockReserved},
			now,
			domain.StatusPaymentPending,
			paymentCutoff,
		).
		Order("created_at").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list orders for reconciliation: %w", err)
	}
	result := make([]*domain.Order, 0, len(records))
	for _, record := range records {
		order, err := r.findOrder(
			r.db.WithContext(ctx), "user_id = ? AND id = ?", record.UserID, record.ID,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, order)
	}
	return result, nil
}

func (r *Repository) findOrder(tx *gorm.DB, query string, arguments ...any) (*domain.Order, error) {
	var record orderModel
	if err := tx.Table("orders.orders").Where(query, arguments...).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("find order: %w", err)
	}
	var itemRecords []orderItemModel
	if err := tx.Table("orders.order_items").Where("order_id = ?", record.ID).
		Order("created_at, id").Find(&itemRecords).Error; err != nil {
		return nil, fmt.Errorf("find order items: %w", err)
	}
	result := orderDomain(record)
	result.Items = make([]domain.Item, 0, len(itemRecords))
	for _, item := range itemRecords {
		result.Items = append(result.Items, orderItemDomain(item))
	}
	return result, nil
}

func cartItemRecord(item *domain.CartItem) cartItemModel {
	return cartItemModel{
		ID: item.ID, UserID: item.UserID, BookID: item.BookID, Quantity: item.Quantity,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func cartItemDomain(item cartItemModel) *domain.CartItem {
	return &domain.CartItem{
		ID: item.ID, UserID: item.UserID, BookID: item.BookID, Quantity: item.Quantity,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func orderRecord(order *domain.Order) orderModel {
	var paymentID *string
	if order.PaymentID != "" {
		paymentID = &order.PaymentID
	}
	return orderModel{
		ID: order.ID, UserID: order.UserID, Status: order.Status, TotalCents: order.TotalCents,
		Currency: order.Currency, PaymentID: paymentID, FailureReason: order.FailureReason,
		IdempotencyKey: order.IdempotencyKey, CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt,
		ReservationExpiresAt: order.ReservationExpiresAt,
	}
}

func orderDomain(order orderModel) *domain.Order {
	paymentID := ""
	if order.PaymentID != nil {
		paymentID = *order.PaymentID
	}
	return &domain.Order{
		ID: order.ID, UserID: order.UserID, Status: order.Status, TotalCents: order.TotalCents,
		Currency: order.Currency, PaymentID: paymentID, FailureReason: order.FailureReason,
		IdempotencyKey: order.IdempotencyKey, CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt,
		ReservationExpiresAt: order.ReservationExpiresAt,
	}
}

func orderItemRecord(orderID string, item domain.Item, createdAt time.Time) orderItemModel {
	return orderItemModel{
		ID: item.ID, OrderID: orderID, BookID: item.BookID, SellerID: item.SellerID,
		Title: item.Title, UnitPriceCents: item.UnitPriceCents, Quantity: item.Quantity,
		SubtotalCents: item.SubtotalCents, CreatedAt: createdAt,
	}
}

func orderItemDomain(item orderItemModel) domain.Item {
	return domain.Item{
		ID: item.ID, BookID: item.BookID, SellerID: item.SellerID, Title: item.Title,
		UnitPriceCents: item.UnitPriceCents, Quantity: item.Quantity, SubtotalCents: item.SubtotalCents,
	}
}
