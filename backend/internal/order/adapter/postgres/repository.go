package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	customeractivity "github.com/thinhnguyenwilliam/book-store/backend/internal/events/customeractivity"
	orderevent "github.com/thinhnguyenwilliam/book-store/backend/internal/events/order"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/order/domain"
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
	ID             string  `gorm:"type:uuid;primaryKey"`
	OrderID        string  `gorm:"type:uuid;not null"`
	CartItemID     *string `gorm:"type:uuid"`
	CartUpdatedAt  *time.Time
	BookID         string    `gorm:"type:uuid;not null"`
	SellerID       string    `gorm:"not null"`
	Title          string    `gorm:"not null"`
	UnitPriceCents int64     `gorm:"not null"`
	Quantity       int32     `gorm:"not null"`
	SubtotalCents  int64     `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

type orderOutboxModel struct {
	ID          string    `gorm:"type:uuid;primaryKey"`
	AggregateID string    `gorm:"type:uuid;not null"`
	EventType   string    `gorm:"not null"`
	TraceID     string    `gorm:"not null"`
	Payload     []byte    `gorm:"type:jsonb;not null"`
	AvailableAt time.Time `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
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

func (r *Repository) ClearOrderedCartItems(ctx context.Context, userID string, items []domain.Item) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if item.CartItemID == "" || item.CartUpdatedAt.IsZero() {
				continue
			}
			// Delete only the exact cart snapshot used to price this order. If the
			// customer changed the quantity while checkout was running, preserve it.
			result := tx.Table("orders.cart_items").
				Where("id = ? AND user_id = ? AND quantity = ? AND updated_at = ?",
					item.CartItemID, userID, item.Quantity, item.CartUpdatedAt).
				Delete(&cartItemModel{})
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("clear ordered cart items: %w", err)
	}
	return nil
}

func (r *Repository) RestoreCartItems(
	ctx context.Context,
	userID string,
	items []domain.Item,
	now time.Time,
) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			id := item.CartItemID
			if id == "" {
				id = uuid.NewString()
			}
			record := cartItemModel{
				ID: id, UserID: userID, BookID: item.BookID, Quantity: item.Quantity,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Table("orders.cart_items").Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}, {Name: "book_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"quantity":   gorm.Expr("GREATEST(cart_items.quantity, EXCLUDED.quantity)"),
					"updated_at": now,
				}),
			}).Create(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("restore cart items: %w", err)
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
		if err := tx.Table("orders.order_items").Create(&items).Error; err != nil {
			return err
		}
		return writeOrderEvent(ctx, tx, order, "", orderevent.EventCreated, "", order.CreatedAt)
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
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current orderModel
		if err := tx.Table("orders.orders").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", userID, id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrOrderNotFound
			}
			return err
		}
		if current.Status == status {
			return nil
		}
		if !containsStatus(allowedStatuses, current.Status) {
			return domain.ErrOrderState
		}
		updates := map[string]any{"status": status, "failure_reason": failureReason, "updated_at": now}
		if paymentID != "" {
			updates["payment_id"] = paymentID
			current.PaymentID = &paymentID
		}
		if err := tx.Table("orders.orders").Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		previousStatus := current.Status
		current.Status, current.FailureReason, current.UpdatedAt = status, failureReason, now
		return writeOrderEvent(
			ctx, tx, orderDomain(current), previousStatus, eventTypeForStatus(status),
			failureStage(previousStatus, status, failureReason), now,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("update order state: %w", err)
	}
	return r.FindOrder(ctx, userID, id)
}

func (r *Repository) BeginPayment(
	ctx context.Context,
	userID, id string,
	now time.Time,
) (*domain.Order, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current orderModel
		if err := tx.Table("orders.orders").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", userID, id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrOrderNotFound
			}
			return err
		}
		if current.Status == domain.StatusPaymentPending {
			return nil
		}
		if current.Status != domain.StatusStockReserved {
			return domain.ErrOrderState
		}
		if !current.ReservationExpiresAt.After(now) {
			return domain.ErrReservationExpired
		}
		previousStatus := current.Status
		current.Status = domain.StatusPaymentPending
		current.UpdatedAt = now
		if err := tx.Table("orders.orders").Where("id = ?", id).
			Updates(map[string]any{"status": current.Status, "updated_at": now}).Error; err != nil {
			return err
		}
		return writeOrderEvent(
			ctx, tx, orderDomain(current), previousStatus,
			orderevent.EventPaymentPending, "", now,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("begin order payment: %w", err)
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
	var cartItemID *string
	var cartUpdatedAt *time.Time
	if item.CartItemID != "" {
		cartItemID = &item.CartItemID
	}
	if !item.CartUpdatedAt.IsZero() {
		cartUpdatedAt = &item.CartUpdatedAt
	}
	return orderItemModel{
		ID: item.ID, OrderID: orderID, CartItemID: cartItemID, CartUpdatedAt: cartUpdatedAt,
		BookID: item.BookID, SellerID: item.SellerID,
		Title: item.Title, UnitPriceCents: item.UnitPriceCents, Quantity: item.Quantity,
		SubtotalCents: item.SubtotalCents, CreatedAt: createdAt,
	}
}

func orderItemDomain(item orderItemModel) domain.Item {
	result := domain.Item{
		ID: item.ID, BookID: item.BookID, SellerID: item.SellerID, Title: item.Title,
		UnitPriceCents: item.UnitPriceCents, Quantity: item.Quantity, SubtotalCents: item.SubtotalCents,
	}
	if item.CartItemID != nil {
		result.CartItemID = *item.CartItemID
	}
	if item.CartUpdatedAt != nil {
		result.CartUpdatedAt = *item.CartUpdatedAt
	}
	return result
}

func writeOrderEvent(
	ctx context.Context,
	tx *gorm.DB,
	order *domain.Order,
	previousStatus, eventType, stage string,
	occurredAt time.Time,
) error {
	eventID := uuid.NewString()
	event := orderevent.Event{
		EventID: eventID, EventType: eventType, SchemaVersion: orderevent.SchemaVersion,
		AggregateType: "order", AggregateID: order.ID, OccurredAt: occurredAt.UTC(),
		TraceID: apptrace.IDFromContext(ctx), PreviousStatus: previousStatus, FailureStage: stage,
		Order: orderevent.Snapshot{
			ID: order.ID, UserID: order.UserID, Status: order.Status, TotalCents: order.TotalCents,
			Currency: order.Currency, PaymentID: order.PaymentID, FailureReason: order.FailureReason,
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal order integration event: %w", err)
	}
	model := orderOutboxModel{
		ID: eventID, AggregateID: order.ID, EventType: eventType, TraceID: event.TraceID,
		Payload: payload, AvailableAt: occurredAt.UTC(), CreatedAt: occurredAt.UTC(),
	}
	if err := tx.Table("orders.outbox_events").Create(&model).Error; err != nil {
		return fmt.Errorf("create order outbox event: %w", err)
	}
	if eventType == orderevent.EventConfirmed {
		activityID := uuid.NewString()
		activity := customeractivity.Event{
			EventID: activityID, EventType: customeractivity.EventOrderConfirmed,
			SchemaVersion: customeractivity.SchemaVersion, ActorID: order.UserID,
			UserID: order.UserID, OrderID: order.ID, Source: "order-service",
			OccurredAt: occurredAt.UTC(), TraceID: event.TraceID,
		}
		activityPayload, marshalErr := json.Marshal(activity)
		if marshalErr != nil {
			return fmt.Errorf("marshal confirmed customer activity: %w", marshalErr)
		}
		activityOutbox := orderOutboxModel{
			ID: activityID, AggregateID: order.UserID, EventType: activity.EventType,
			TraceID: activity.TraceID, Payload: activityPayload,
			AvailableAt: occurredAt.UTC(), CreatedAt: occurredAt.UTC(),
		}
		if err := tx.Table("orders.customer_activity_outbox_events").Create(&activityOutbox).Error; err != nil {
			return fmt.Errorf("create order customer activity outbox event: %w", err)
		}
	}
	return nil
}

func eventTypeForStatus(status string) string {
	switch status {
	case domain.StatusStockReserved:
		return orderevent.EventStockReserved
	case domain.StatusPaymentPending:
		return orderevent.EventPaymentPending
	case domain.StatusConfirmed:
		return orderevent.EventConfirmed
	case domain.StatusCancelled:
		return orderevent.EventCancelled
	case domain.StatusCompensationPending:
		return orderevent.EventCompensationPending
	default:
		return "order.status_changed"
	}
}

func failureStage(previousStatus, status, reason string) string {
	if status == domain.StatusCompensationPending {
		return "stock_commit"
	}
	if status != domain.StatusCancelled {
		return ""
	}
	if reason == "cancelled by customer" {
		return "customer"
	}
	switch previousStatus {
	case domain.StatusPending, domain.StatusStockReserved:
		return "stock_reservation"
	case domain.StatusPaymentPending:
		return "payment"
	case domain.StatusCompensationPending:
		return "compensation"
	default:
		return "unknown"
	}
}

func containsStatus(statuses []string, candidate string) bool {
	for _, status := range statuses {
		if status == candidate {
			return true
		}
	}
	return false
}
