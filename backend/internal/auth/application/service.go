package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type AuthResult struct {
	AccessToken string
	UserID      string
	ExpiresIn   int64
}

type Service struct {
	repository AccountRepository
	hasher     PasswordHasher
	tokens     TokenManager
	now        func() time.Time
}

func NewService(repository AccountRepository, hasher PasswordHasher, tokens TokenManager) *Service {
	return &Service{repository: repository, hasher: hasher, tokens: tokens, now: time.Now}
}

func (s *Service) Register(ctx context.Context, email, password, displayName string) (AuthResult, error) {
	email = domain.NormalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	if !validEmail(email) || len(password) < 8 || len(displayName) > 100 {
		return AuthResult{}, domain.ErrInvalidInput
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	account := &domain.Account{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		Roles:        []string{"customer"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repository.Create(ctx, account, ProfileRegistration{DisplayName: displayName}); err != nil {
		return AuthResult{}, err
	}

	return s.issue(account)
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	account, err := s.repository.FindByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return AuthResult{}, domain.ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	if err := s.hasher.Compare(account.PasswordHash, password); err != nil {
		return AuthResult{}, domain.ErrInvalidCredentials
	}

	return s.issue(account)
}

func (s *Service) VerifyToken(_ context.Context, token string) (Claims, error) {
	if strings.TrimSpace(token) == "" {
		return Claims{}, domain.ErrInvalidToken
	}
	return s.tokens.Verify(token)
}

func (s *Service) issue(account *domain.Account) (AuthResult, error) {
	token, expiresAt, err := s.tokens.Issue(Claims{
		UserID: account.ID,
		Email:  account.Email,
		Roles:  account.Roles,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}

	return AuthResult{
		AccessToken: token,
		UserID:      account.ID,
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
	}, nil
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
