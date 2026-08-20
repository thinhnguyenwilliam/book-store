package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type accountRepositoryStub struct {
	created *domain.Account
}

func (r *accountRepositoryStub) Create(_ context.Context, account *domain.Account, _ ProfileRegistration) error {
	r.created = account
	return nil
}

func (r *accountRepositoryStub) FindByEmail(_ context.Context, _ string) (*domain.Account, error) {
	return nil, domain.ErrInvalidCredentials
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

func TestRegisterNormalizesEmailAndHashesPassword(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := NewService(repository, passwordHasherStub{}, tokenManagerStub{})

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
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	service := NewService(&accountRepositoryStub{}, passwordHasherStub{}, tokenManagerStub{})

	_, err := service.Register(context.Background(), "reader@example.com", "short", "Reader")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrInvalidInput)
	}
}

func TestRegisterRejectsDisplayNameOverLimit(t *testing.T) {
	service := NewService(&accountRepositoryStub{}, passwordHasherStub{}, tokenManagerStub{})

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
