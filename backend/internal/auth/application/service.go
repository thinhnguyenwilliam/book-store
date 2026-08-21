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
	AccessToken      string
	RefreshToken     string
	UserID           string
	ExpiresIn        int64
	RefreshExpiresIn int64
}

type Service struct {
	repository    AccountRepository
	hasher        PasswordHasher
	accessTokens  TokenManager
	refreshTokens RefreshTokenManager
	identities    IdentityVerifier
	refreshTTL    time.Duration
	now           func() time.Time
}

func NewService(
	repository AccountRepository,
	hasher PasswordHasher,
	accessTokens TokenManager,
	refreshTokens RefreshTokenManager,
	identities IdentityVerifier,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		repository:    repository,
		hasher:        hasher,
		accessTokens:  accessTokens,
		refreshTokens: refreshTokens,
		identities:    identities,
		refreshTTL:    refreshTTL,
		now:           time.Now,
	}
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
	result, session, err := s.issueSession(account, now)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.repository.Create(ctx, account, ProfileRegistration{DisplayName: displayName}, session, nil); err != nil {
		return AuthResult{}, err
	}
	return result, nil
}

func (s *Service) LoginWithGoogle(ctx context.Context, credential string, createAccount bool) (AuthResult, error) {
	if s.identities == nil {
		return AuthResult{}, domain.ErrIdentityUnavailable
	}
	verified, err := s.identities.Verify(ctx, strings.TrimSpace(credential))
	if err != nil {
		if errors.Is(err, domain.ErrIdentityUnavailable) {
			return AuthResult{}, err
		}
		return AuthResult{}, domain.ErrInvalidIdentity
	}
	verified.Email = domain.NormalizeEmail(verified.Email)
	verified.DisplayName = strings.TrimSpace(verified.DisplayName)
	if verified.Provider != domain.IdentityProviderGoogle ||
		strings.TrimSpace(verified.Subject) == "" ||
		!verified.EmailVerified ||
		!validEmail(verified.Email) {
		return AuthResult{}, domain.ErrInvalidIdentity
	}

	account, err := s.repository.FindByIdentity(ctx, verified.Provider, verified.Subject)
	if err == nil {
		return s.createSession(ctx, account)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return AuthResult{}, err
	}

	account, err = s.repository.FindByEmail(ctx, verified.Email)
	if err == nil {
		if !verified.EmailAuthoritative {
			return AuthResult{}, domain.ErrIdentityConflict
		}
		now := s.now().UTC()
		result, session, issueErr := s.issueSession(account, now)
		if issueErr != nil {
			return AuthResult{}, issueErr
		}
		identity := &domain.Identity{
			Provider:  verified.Provider,
			Subject:   verified.Subject,
			AccountID: account.ID,
			Email:     verified.Email,
			CreatedAt: now,
		}
		if err := s.repository.LinkIdentity(ctx, identity, session); err != nil {
			return AuthResult{}, err
		}
		return result, nil
	}
	if !errors.Is(err, domain.ErrInvalidCredentials) && !errors.Is(err, domain.ErrNotFound) {
		return AuthResult{}, err
	}
	if !createAccount {
		return AuthResult{}, domain.ErrInvalidCredentials
	}

	now := s.now().UTC()
	account = &domain.Account{
		ID:        uuid.NewString(),
		Email:     verified.Email,
		Roles:     []string{"customer"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	result, session, err := s.issueSession(account, now)
	if err != nil {
		return AuthResult{}, err
	}
	identity := &domain.Identity{
		Provider:  verified.Provider,
		Subject:   verified.Subject,
		AccountID: account.ID,
		Email:     verified.Email,
		CreatedAt: now,
	}
	if err := s.repository.Create(
		ctx,
		account,
		ProfileRegistration{DisplayName: googleDisplayName(verified.DisplayName, verified.Email)},
		session,
		identity,
	); err != nil {
		return AuthResult{}, err
	}
	return result, nil
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

	return s.createSession(ctx, account)
}

func (s *Service) createSession(ctx context.Context, account *domain.Account) (AuthResult, error) {
	now := s.now().UTC()
	result, session, err := s.issueSession(account, now)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.repository.CreateRefreshSession(ctx, session); err != nil {
		return AuthResult{}, err
	}
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (AuthResult, error) {
	if strings.TrimSpace(rawRefreshToken) == "" {
		return AuthResult{}, domain.ErrInvalidRefreshToken
	}

	now := s.now().UTC()
	rawReplacement, replacementHash, err := s.refreshTokens.Generate()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate refresh token: %w", err)
	}
	replacement := &domain.RefreshSession{
		ID:        uuid.NewString(),
		TokenHash: replacementHash,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	account, err := s.repository.RotateRefreshSession(
		ctx,
		s.refreshTokens.Hash(rawRefreshToken),
		replacement,
		now,
	)
	if err != nil {
		return AuthResult{}, err
	}

	accessToken, expiresAt, err := s.issueAccessToken(account)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		AccessToken:      accessToken,
		RefreshToken:     rawReplacement,
		UserID:           account.ID,
		ExpiresIn:        secondsUntil(now, expiresAt),
		RefreshExpiresIn: secondsUntil(now, replacement.ExpiresAt),
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if strings.TrimSpace(rawRefreshToken) == "" {
		return nil
	}
	return s.repository.RevokeRefreshSession(
		ctx,
		s.refreshTokens.Hash(rawRefreshToken),
		s.now().UTC(),
	)
}

func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrInvalidInput
	}
	return s.repository.Delete(ctx, id, s.now().UTC())
}

func (s *Service) VerifyToken(ctx context.Context, token string) (Claims, error) {
	if strings.TrimSpace(token) == "" {
		return Claims{}, domain.ErrInvalidToken
	}
	claims, err := s.accessTokens.Verify(token)
	if err != nil {
		return Claims{}, err
	}
	account, err := s.repository.FindByID(ctx, claims.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return Claims{}, domain.ErrInvalidToken
	}
	if err != nil {
		return Claims{}, err
	}
	return Claims{UserID: account.ID, Email: account.Email, Roles: account.Roles}, nil
}

func (s *Service) issueSession(account *domain.Account, now time.Time) (AuthResult, *domain.RefreshSession, error) {
	accessToken, expiresAt, err := s.issueAccessToken(account)
	if err != nil {
		return AuthResult{}, nil, err
	}
	rawRefreshToken, refreshHash, err := s.refreshTokens.Generate()
	if err != nil {
		return AuthResult{}, nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshExpiresAt := now.Add(s.refreshTTL)
	session := &domain.RefreshSession{
		ID:        uuid.NewString(),
		AccountID: account.ID,
		FamilyID:  uuid.NewString(),
		TokenHash: refreshHash,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: now,
	}
	return AuthResult{
		AccessToken:      accessToken,
		RefreshToken:     rawRefreshToken,
		UserID:           account.ID,
		ExpiresIn:        secondsUntil(now, expiresAt),
		RefreshExpiresIn: secondsUntil(now, refreshExpiresAt),
	}, session, nil
}

func (s *Service) issueAccessToken(account *domain.Account) (string, time.Time, error) {
	token, expiresAt, err := s.accessTokens.Issue(Claims{
		UserID: account.ID,
		Email:  account.Email,
		Roles:  account.Roles,
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("issue access token: %w", err)
	}
	return token, expiresAt, nil
}

func secondsUntil(now, expiresAt time.Time) int64 {
	seconds := int64(expiresAt.Sub(now).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func googleDisplayName(displayName, email string) string {
	displayName = strings.TrimSpace(displayName)
	runes := []rune(displayName)
	if len(runes) > 100 {
		displayName = string(runes[:100])
	}
	if displayName != "" {
		return displayName
	}
	localPart, _, ok := strings.Cut(email, "@")
	if ok && localPart != "" {
		return localPart
	}
	return "Google user"
}
