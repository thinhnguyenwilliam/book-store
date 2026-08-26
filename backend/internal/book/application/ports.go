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
	ReserveStock(ctx context.Context, reservation *domain.StockReservation) (*domain.StockReservation, error)
	CommitStock(ctx context.Context, orderID, bookID string, now time.Time) (*domain.StockReservation, error)
	ReleaseStock(ctx context.Context, orderID, bookID string, now time.Time) (*domain.StockReservation, error)
}

type Cache interface {
	GetJSON(ctx context.Context, key string, destination any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Version(ctx context.Context, key string) (int64, error)
	BumpVersion(ctx context.Context, key string) error
	TryLock(ctx context.Context, key string, ttl time.Duration) (token string, locked bool, err error)
	Unlock(ctx context.Context, key, token string) error
}
