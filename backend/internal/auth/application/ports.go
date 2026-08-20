package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account, profile ProfileRegistration) error
	FindByEmail(ctx context.Context, email string) (*domain.Account, error)
}

type ProfileRegistration struct {
	DisplayName string
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type Claims struct {
	UserID string
	Email  string
	Roles  []string
}

type TokenManager interface {
	Issue(claims Claims) (token string, expiresAt time.Time, err error)
	Verify(token string) (Claims, error)
}
