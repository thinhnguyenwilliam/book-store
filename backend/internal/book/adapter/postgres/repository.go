package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type bookModel struct {
	ID         string    `gorm:"type:uuid;primaryKey"`
	Title      string    `gorm:"not null"`
	Author     string    `gorm:"not null"`
	ISBN       string    `gorm:"column:isbn;not null;uniqueIndex"`
	PriceCents int64     `gorm:"not null"`
	Stock      int32     `gorm:"not null"`
	SellerID   *string   `gorm:"type:uuid"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

type stockReservationModel struct {
	ID             string    `gorm:"type:uuid;primaryKey"`
	OrderID        string    `gorm:"type:uuid;not null"`
	BookID         string    `gorm:"type:uuid;not null"`
	Quantity       int32     `gorm:"not null"`
	Status         string    `gorm:"not null"`
	IdempotencyKey string    `gorm:"not null"`
	ExpiresAt      time.Time `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, book *domain.Book) error {
	record := toModel(book)
	err := r.db.WithContext(ctx).Table("catalog.books").Create(&record).Error
	return mapWriteError(err)
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Book, error) {
	var record bookModel
	err := r.db.WithContext(ctx).
		Table("catalog.books").
		Where("id = ?", id).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find book: %w", err)
	}
	return toDomain(record), nil
}

func (r *Repository) List(ctx context.Context, limit int32, cursor *application.BookCursor) ([]*domain.Book, error) {
	db := r.db.WithContext(ctx).
		Table("catalog.books").
		Order("created_at DESC").
		Order("id DESC").
		Limit(int(limit))
	if cursor != nil {
		db = db.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}
	var records []bookModel
	if err := db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}

	books := make([]*domain.Book, 0, len(records))
	for _, record := range records {
		books = append(books, toDomain(record))
	}
	return books, nil
}

func (r *Repository) Update(ctx context.Context, book *domain.Book) error {
	result := r.db.WithContext(ctx).
		Table("catalog.books").
		Where("id = ?", book.ID).
		Updates(map[string]any{
			"title":       book.Title,
			"author":      book.Author,
			"isbn":        book.ISBN,
			"price_cents": book.PriceCents,
			"stock":       book.Stock,
			"seller_id":   nullableUUID(book.SellerID),
			"updated_at":  book.UpdatedAt,
		})
	if err := mapWriteError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ReserveStock(
	ctx context.Context,
	reservation *domain.StockReservation,
) (*domain.StockReservation, error) {
	var result stockReservationModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("catalog.stock_reservations").
			Where("order_id = ? AND book_id = ?", reservation.OrderID, reservation.BookID).
			First(&result)
		if query.Error == nil {
			if result.Quantity != reservation.Quantity {
				return domain.ErrReservationState
			}
			return nil
		}
		if !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return query.Error
		}

		var book bookModel
		if err := tx.Table("catalog.books").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", reservation.BookID).First(&book).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}

		query = tx.Table("catalog.stock_reservations").
			Where("order_id = ? AND book_id = ?", reservation.OrderID, reservation.BookID).
			First(&result)
		if query.Error == nil {
			if result.Quantity != reservation.Quantity {
				return domain.ErrReservationState
			}
			return nil
		}
		if !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return query.Error
		}
		if book.Stock < reservation.Quantity {
			return domain.ErrInsufficientStock
		}
		if err := tx.Table("catalog.books").Where("id = ?", reservation.BookID).
			Update("stock", gorm.Expr("stock - ?", reservation.Quantity)).Error; err != nil {
			return err
		}
		result = stockReservationRecord(reservation)
		return tx.Table("catalog.stock_reservations").Create(&result).Error
	})
	if err != nil {
		return nil, fmt.Errorf("reserve stock: %w", err)
	}
	return stockReservationDomain(result), nil
}

func (r *Repository) CommitStock(
	ctx context.Context,
	orderID, bookID string,
	now time.Time,
) (*domain.StockReservation, error) {
	return r.transitionReservation(ctx, orderID, bookID, "committed", now, false)
}

func (r *Repository) ReleaseStock(
	ctx context.Context,
	orderID, bookID string,
	now time.Time,
) (*domain.StockReservation, error) {
	return r.transitionReservation(ctx, orderID, bookID, "released", now, true)
}

func (r *Repository) transitionReservation(
	ctx context.Context,
	orderID, bookID, target string,
	now time.Time,
	restoreStock bool,
) (*domain.StockReservation, error) {
	var record stockReservationModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("catalog.stock_reservations").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_id = ? AND book_id = ?", orderID, bookID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrReservationMissing
			}
			return err
		}
		if record.Status == target {
			return nil
		}
		if record.Status == "released" {
			return domain.ErrReservationState
		}
		if target == "committed" && record.Status != "reserved" {
			return domain.ErrReservationState
		}
		if restoreStock {
			if err := tx.Table("catalog.books").Where("id = ?", bookID).
				Update("stock", gorm.Expr("stock + ?", record.Quantity)).Error; err != nil {
				return err
			}
		}
		record.Status = target
		record.UpdatedAt = now
		return tx.Table("catalog.stock_reservations").Where("id = ?", record.ID).
			Updates(map[string]any{"status": target, "updated_at": now}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("transition stock reservation: %w", err)
	}
	return stockReservationDomain(record), nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Table("catalog.books").
		Where("id = ?", id).
		Delete(&bookModel{})
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrForeignKeyViolated) {
			return domain.ErrBookInUse
		}
		return fmt.Errorf("delete book: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrISBNExists
	}
	return fmt.Errorf("write book: %w", err)
}

func toModel(book *domain.Book) bookModel {
	return bookModel{
		ID:         book.ID,
		Title:      book.Title,
		Author:     book.Author,
		ISBN:       book.ISBN,
		PriceCents: book.PriceCents,
		Stock:      book.Stock,
		SellerID:   stringPointer(book.SellerID),
		CreatedAt:  book.CreatedAt,
		UpdatedAt:  book.UpdatedAt,
	}
}

func toDomain(record bookModel) *domain.Book {
	return &domain.Book{
		ID:         record.ID,
		Title:      record.Title,
		Author:     record.Author,
		ISBN:       record.ISBN,
		PriceCents: record.PriceCents,
		Stock:      record.Stock,
		SellerID:   stringValue(record.SellerID),
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stockReservationRecord(value *domain.StockReservation) stockReservationModel {
	return stockReservationModel{
		ID:             value.ID,
		OrderID:        value.OrderID,
		BookID:         value.BookID,
		Quantity:       value.Quantity,
		Status:         value.Status,
		IdempotencyKey: value.IdempotencyKey,
		ExpiresAt:      value.ExpiresAt,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
	}
}

func stockReservationDomain(value stockReservationModel) *domain.StockReservation {
	return &domain.StockReservation{
		ID:             value.ID,
		OrderID:        value.OrderID,
		BookID:         value.BookID,
		Quantity:       value.Quantity,
		Status:         value.Status,
		IdempotencyKey: value.IdempotencyKey,
		ExpiresAt:      value.ExpiresAt,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
	}
}
