package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/comment/domain"
)

type Repository interface {
	Create(context.Context, *domain.Comment) error
	FindByID(context.Context, string) (*domain.Comment, error)
	ListRoots(context.Context, string, int32, *domain.Cursor) ([]*domain.Comment, error)
	ListReplies(context.Context, string, int32, *domain.Cursor) ([]*domain.Comment, error)
	Update(context.Context, string, string, string, time.Time) (*domain.Comment, error)
	SoftDelete(context.Context, string, time.Time) (*domain.Comment, error)
	Moderate(context.Context, string, string, time.Time) (*domain.Comment, error)
}

type BookResolver interface {
	Exists(context.Context, string) error
}
type AuthorResolver interface {
	DisplayName(context.Context, string) (string, error)
}
