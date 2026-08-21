package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidInput        = errors.New("invalid account input")
	ErrInvalidToken        = errors.New("invalid access token")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenReused  = errors.New("refresh token reuse detected")
	ErrNotFound            = errors.New("account not found")
	ErrInvalidIdentity     = errors.New("invalid external identity credential")
	ErrIdentityConflict    = errors.New("external identity conflicts with an existing account")
	ErrIdentityUnavailable = errors.New("external identity provider is not configured")
	ErrIdentityProvider    = errors.New("external identity provider is unavailable")
)

const IdentityProviderGoogle = "google"
const IdentityProviderFacebook = "facebook"

type Account struct {
	ID           string
	Email        string
	PasswordHash string
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshSession struct {
	ID           string
	AccountID    string
	FamilyID     string
	TokenHash    string
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	ReplacedByID *string
	LastUsedAt   *time.Time
	CreatedAt    time.Time
}

type Identity struct {
	Provider  string
	Subject   string
	AccountID string
	Email     string
	CreatedAt time.Time
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
