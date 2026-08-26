package application

import (
	"context"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type AccountRepository interface {
	Create(
		ctx context.Context,
		account *domain.Account,
		profile ProfileRegistration,
		session *domain.RefreshSession,
		identity *domain.Identity,
	) error
	FindByID(ctx context.Context, id string) (*domain.Account, error)
	FindByEmail(ctx context.Context, email string) (*domain.Account, error)
	FindByIdentity(ctx context.Context, provider, subject string) (*domain.Account, error)
	LinkIdentity(ctx context.Context, identity *domain.Identity, session *domain.RefreshSession) error
	Delete(ctx context.Context, id string, deletedAt time.Time) error
	CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error
	RotateRefreshSession(ctx context.Context, tokenHash string, replacement *domain.RefreshSession, now time.Time) (*domain.Account, error)
	RevokeRefreshSession(ctx context.Context, tokenHash string, now time.Time) error
}

type ProfileRegistration struct {
	DisplayName string
}

type VerifiedIdentity struct {
	Provider           string
	Subject            string
	Email              string
	DisplayName        string
	EmailVerified      bool
	EmailAuthoritative bool
}

type IdentityVerifier interface {
	Verify(ctx context.Context, credential, expectedNonce string) (VerifiedIdentity, error)
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
