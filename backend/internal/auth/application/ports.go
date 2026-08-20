package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account, profile ProfileRegistration, session *domain.RefreshSession) error
	FindByID(ctx context.Context, id string) (*domain.Account, error)
	FindByEmail(ctx context.Context, email string) (*domain.Account, error)
	Delete(ctx context.Context, id string, deletedAt time.Time) error
	CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error
	RotateRefreshSession(ctx context.Context, tokenHash string, replacement *domain.RefreshSession, now time.Time) (*domain.Account, error)
	RevokeRefreshSession(ctx context.Context, tokenHash string, now time.Time) error
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

type RefreshTokenManager interface {
	Generate() (rawToken string, tokenHash string, err error)
	Hash(rawToken string) string
}
