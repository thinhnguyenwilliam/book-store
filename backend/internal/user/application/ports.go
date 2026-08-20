package application

import (
	"context"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/user/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}
