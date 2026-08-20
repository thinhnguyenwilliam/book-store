package application

import (
	"context"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/book/domain"
)

type BookRepository interface {
	Create(ctx context.Context, book *domain.Book) error
	FindByID(ctx context.Context, id string) (*domain.Book, error)
	List(ctx context.Context, limit, offset int32) ([]*domain.Book, int64, error)
	Update(ctx context.Context, book *domain.Book) error
	Delete(ctx context.Context, id string) error
}
