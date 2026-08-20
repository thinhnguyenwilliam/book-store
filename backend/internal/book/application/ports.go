package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
)

type BookCursor struct {
	CreatedAt time.Time
	ID        string
}

type BookRepository interface {
	Create(ctx context.Context, book *domain.Book) error
	FindByID(ctx context.Context, id string) (*domain.Book, error)
	List(ctx context.Context, limit int32, cursor *BookCursor) ([]*domain.Book, error)
	Update(ctx context.Context, book *domain.Book) error
	Delete(ctx context.Context, id string) error
}
