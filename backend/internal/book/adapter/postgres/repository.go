package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
	"gorm.io/gorm"
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
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
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

func (r *Repository) List(ctx context.Context, limit, offset int32) ([]*domain.Book, int64, error) {
	db := r.db.WithContext(ctx).Table("catalog.books")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count books: %w", err)
	}

	var records []bookModel
	if err := db.Order("created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list books: %w", err)
	}

	books := make([]*domain.Book, 0, len(records))
	for _, record := range records {
		books = append(books, toDomain(record))
	}
	return books, total, nil
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

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Table("catalog.books").
		Where("id = ?", id).
		Delete(&bookModel{})
	if result.Error != nil {
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
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}
}
