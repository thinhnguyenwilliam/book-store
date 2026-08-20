package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type accountRepositoryStub struct {
	created        *domain.Account
	createdSession *domain.RefreshSession
	account        *domain.Account
	revokedHash    string
}

func (r *accountRepositoryStub) Create(
	_ context.Context,
	account *domain.Account,
	_ ProfileRegistration,
	session *domain.RefreshSession,
) error {
	r.created = account
	r.createdSession = session
	return nil
}

func (r *accountRepositoryStub) FindByEmail(_ context.Context, _ string) (*domain.Account, error) {
	if r.account != nil {
		return r.account, nil
	}
	return nil, domain.ErrInvalidCredentials
}

func (r *accountRepositoryStub) CreateRefreshSession(_ context.Context, session *domain.RefreshSession) error {
	r.createdSession = session
	return nil
}

func (r *accountRepositoryStub) RotateRefreshSession(
	_ context.Context,
	_ string,
	replacement *domain.RefreshSession,
	_ time.Time,
) (*domain.Account, error) {
	replacement.AccountID = r.account.ID
	replacement.FamilyID = "family-id"
	r.createdSession = replacement
	return r.account, nil
}

func (r *accountRepositoryStub) RevokeRefreshSession(_ context.Context, tokenHash string, _ time.Time) error {
	r.revokedHash = tokenHash
	return nil
}

type passwordHasherStub struct{}

func (passwordHasherStub) Hash(password string) (string, error) { return "hashed:" + password, nil }
func (passwordHasherStub) Compare(hash, password string) error {
	if hash != "hashed:"+password {
		return errors.New("mismatch")
	}
	return nil
}

type tokenManagerStub struct{}

func (tokenManagerStub) Issue(Claims) (string, time.Time, error) {
	return "access-token", time.Now().Add(15 * time.Minute), nil
}

func (tokenManagerStub) Verify(string) (Claims, error) { return Claims{}, nil }

type refreshTokenManagerStub struct{}

func (refreshTokenManagerStub) Generate() (string, string, error) {
	return "refresh-token", "hash:refresh-token", nil
}

func (refreshTokenManagerStub) Hash(rawToken string) string { return "hash:" + rawToken }

func newTestService(repository AccountRepository) *Service {
	return NewService(
		repository,
		passwordHasherStub{},
		tokenManagerStub{},
		refreshTokenManagerStub{},
		7*24*time.Hour,
	)
}

func TestRegisterNormalizesEmailAndHashesPassword(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newTestService(repository)

	result, err := service.Register(context.Background(), " Reader@Example.com ", "password123", "Reader")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repository.created.Email != "reader@example.com" {
		t.Fatalf("email = %q, want normalized email", repository.created.Email)
	}
	if repository.created.PasswordHash != "hashed:password123" {
		t.Fatalf("password hash was not stored")
	}
	if result.AccessToken != "access-token" || result.UserID == "" {
		t.Fatalf("unexpected auth result: %+v", result)
	}
	if result.RefreshToken != "refresh-token" || repository.createdSession == nil {
		t.Fatalf("refresh session was not created: %+v", result)
	}
	if repository.createdSession.FamilyID == "" {
		t.Fatal("refresh session family was not created")
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	service := newTestService(&accountRepositoryStub{})

	_, err := service.Register(context.Background(), "reader@example.com", "short", "Reader")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrInvalidInput)
	}
}

func TestRegisterRejectsDisplayNameOverLimit(t *testing.T) {
	service := newTestService(&accountRepositoryStub{})

	_, err := service.Register(
		context.Background(),
		"reader@example.com",
		"password123",
		string(make([]byte, 101)),
	)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrInvalidInput)
	}
}

func TestRefreshRotatesSessionAndIssuesAccessToken(t *testing.T) {
	repository := &accountRepositoryStub{account: &domain.Account{
		ID: "user-id", Email: "reader@example.com", Roles: []string{"customer"},
	}}
	service := newTestService(repository)

	result, err := service.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.AccessToken != "access-token" || result.RefreshToken != "refresh-token" {
		t.Fatalf("unexpected refresh result: %+v", result)
	}
	if repository.createdSession == nil || repository.createdSession.AccountID != "user-id" {
		t.Fatalf("replacement session was not created")
	}
}

func TestLogoutHashesAndRevokesRefreshToken(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newTestService(repository)

	if err := service.Logout(context.Background(), "refresh-token"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if repository.revokedHash != "hash:refresh-token" {
		t.Fatalf("revoked hash = %q", repository.revokedHash)
	}
}
