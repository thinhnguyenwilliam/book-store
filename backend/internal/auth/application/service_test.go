package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type accountRepositoryStub struct {
	created         *domain.Account
	createdSession  *domain.RefreshSession
	createdIdentity *domain.Identity
	account         *domain.Account
	identityAccount *domain.Account
	revokedHash     string
	deletedID       string
}

func (r *accountRepositoryStub) Create(
	_ context.Context,
	account *domain.Account,
	_ ProfileRegistration,
	session *domain.RefreshSession,
	identity *domain.Identity,
) error {
	r.created = account
	r.createdSession = session
	r.createdIdentity = identity
	return nil
}

func (r *accountRepositoryStub) FindByIdentity(_ context.Context, _, _ string) (*domain.Account, error) {
	if r.identityAccount != nil {
		return r.identityAccount, nil
	}
	return nil, domain.ErrNotFound
}

func (r *accountRepositoryStub) LinkIdentity(
	_ context.Context,
	identity *domain.Identity,
	session *domain.RefreshSession,
) error {
	r.createdIdentity = identity
	r.createdSession = session
	return nil
}

func (r *accountRepositoryStub) FindByEmail(_ context.Context, _ string) (*domain.Account, error) {
	if r.account != nil {
		return r.account, nil
	}
	return nil, domain.ErrInvalidCredentials
}

func (r *accountRepositoryStub) FindByID(_ context.Context, id string) (*domain.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, domain.ErrNotFound
}

func (r *accountRepositoryStub) Delete(_ context.Context, id string, _ time.Time) error {
	r.deletedID = id
	return nil
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

func (tokenManagerStub) Verify(string) (Claims, error) { return Claims{UserID: "user-id"}, nil }

type refreshTokenManagerStub struct{}

func (refreshTokenManagerStub) Generate() (string, string, error) {
	return "refresh-token", "hash:refresh-token", nil
}

func (refreshTokenManagerStub) Hash(rawToken string) string { return "hash:" + rawToken }

type identityVerifierStub struct {
	identity VerifiedIdentity
	err      error
}

func (v identityVerifierStub) Verify(context.Context, string, string) (VerifiedIdentity, error) {
	return v.identity, v.err
}

func newTestService(repository AccountRepository) *Service {
	return NewService(
		repository,
		passwordHasherStub{},
		tokenManagerStub{},
		refreshTokenManagerStub{},
		map[string]IdentityVerifier{},
		7*24*time.Hour,
	)
}

func newGoogleTestService(repository AccountRepository, identity VerifiedIdentity) *Service {
	return NewService(
		repository,
		passwordHasherStub{},
		tokenManagerStub{},
		refreshTokenManagerStub{},
		map[string]IdentityVerifier{
			domain.IdentityProviderGoogle: identityVerifierStub{identity: identity},
		},
		7*24*time.Hour,
	)
}

func newFacebookTestService(repository AccountRepository, identity VerifiedIdentity) *Service {
	return NewService(
		repository,
		passwordHasherStub{},
		tokenManagerStub{},
		refreshTokenManagerStub{},
		map[string]IdentityVerifier{
			domain.IdentityProviderFacebook: identityVerifierStub{identity: identity},
		},
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

func TestGoogleLoginUsesExistingIdentity(t *testing.T) {
	account := &domain.Account{ID: "user-id", Email: "reader@gmail.com", Roles: []string{"customer"}}
	repository := &accountRepositoryStub{identityAccount: account}
	service := newGoogleTestService(repository, VerifiedIdentity{
		Provider: domain.IdentityProviderGoogle, Subject: "google-sub", Email: account.Email, EmailVerified: true,
	})

	result, err := service.LoginWithGoogle(context.Background(), "google-credential", "state", false)
	if err != nil {
		t.Fatalf("LoginWithGoogle() error = %v", err)
	}
	if result.UserID != account.ID || repository.createdSession == nil {
		t.Fatalf("existing Google identity did not create a session: %+v", result)
	}
}

func TestGoogleLoginCreatesCustomerWithIdentity(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newGoogleTestService(repository, VerifiedIdentity{
		Provider:           domain.IdentityProviderGoogle,
		Subject:            "google-sub",
		Email:              "reader@gmail.com",
		DisplayName:        "Reader",
		EmailVerified:      true,
		EmailAuthoritative: true,
	})

	result, err := service.LoginWithGoogle(context.Background(), "google-credential", "state", true)
	if err != nil {
		t.Fatalf("LoginWithGoogle() error = %v", err)
	}
	if repository.created == nil || repository.created.PasswordHash != "" {
		t.Fatalf("Google account was not created without a password: %+v", repository.created)
	}
	if repository.createdIdentity == nil || repository.createdIdentity.Subject != "google-sub" {
		t.Fatalf("Google identity was not stored: %+v", repository.createdIdentity)
	}
	if result.UserID != repository.created.ID {
		t.Fatalf("result user ID = %q, want %q", result.UserID, repository.created.ID)
	}
}

func TestGoogleLoginDoesNotCreateAccountForAdminFlow(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newGoogleTestService(repository, VerifiedIdentity{
		Provider: domain.IdentityProviderGoogle, Subject: "google-sub", Email: "reader@gmail.com", EmailVerified: true,
	})

	_, err := service.LoginWithGoogle(context.Background(), "google-credential", "state", false)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("LoginWithGoogle() error = %v, want %v", err, domain.ErrInvalidCredentials)
	}
	if repository.created != nil {
		t.Fatal("admin Google login must not create a customer account")
	}
}

func TestGoogleLoginRejectsUnsafeEmailLink(t *testing.T) {
	account := &domain.Account{ID: "user-id", Email: "reader@example.com", Roles: []string{"customer"}}
	repository := &accountRepositoryStub{account: account}
	service := newGoogleTestService(repository, VerifiedIdentity{
		Provider: domain.IdentityProviderGoogle, Subject: "google-sub", Email: account.Email, EmailVerified: true,
	})

	_, err := service.LoginWithGoogle(context.Background(), "google-credential", "state", true)
	if !errors.Is(err, domain.ErrIdentityConflict) {
		t.Fatalf("LoginWithGoogle() error = %v, want %v", err, domain.ErrIdentityConflict)
	}
}

func TestFacebookLoginCreatesCustomerWithIdentity(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newFacebookTestService(repository, VerifiedIdentity{
		Provider:           domain.IdentityProviderFacebook,
		Subject:            "facebook-user-id",
		Email:              "reader@example.com",
		DisplayName:        "Reader Nguyen",
		EmailVerified:      true,
		EmailAuthoritative: true,
	})

	result, err := service.LoginWithFacebook(context.Background(), "facebook-user-token", true)
	if err != nil {
		t.Fatalf("LoginWithFacebook() error = %v", err)
	}
	if repository.created == nil || repository.created.PasswordHash != "" {
		t.Fatalf("Facebook account was not created without a password: %+v", repository.created)
	}
	if repository.createdIdentity == nil || repository.createdIdentity.Provider != domain.IdentityProviderFacebook {
		t.Fatalf("Facebook identity was not stored: %+v", repository.createdIdentity)
	}
	if result.UserID != repository.created.ID {
		t.Fatalf("result user ID = %q, want %q", result.UserID, repository.created.ID)
	}
}

func TestFacebookLoginLinksAuthoritativeEmail(t *testing.T) {
	account := &domain.Account{ID: "user-id", Email: "reader@example.com", Roles: []string{"admin"}}
	repository := &accountRepositoryStub{account: account}
	service := newFacebookTestService(repository, VerifiedIdentity{
		Provider:           domain.IdentityProviderFacebook,
		Subject:            "facebook-user-id",
		Email:              account.Email,
		EmailVerified:      true,
		EmailAuthoritative: true,
	})

	result, err := service.LoginWithFacebook(context.Background(), "facebook-user-token", false)
	if err != nil {
		t.Fatalf("LoginWithFacebook() error = %v", err)
	}
	if result.UserID != account.ID || repository.createdIdentity == nil {
		t.Fatalf("Facebook identity was not linked: result=%+v identity=%+v", result, repository.createdIdentity)
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

func TestDeleteAccountValidatesIDAndCallsRepository(t *testing.T) {
	repository := &accountRepositoryStub{}
	service := newTestService(repository)
	id := "6b92edb2-f406-43dd-851a-ac7acb13cfc2"

	if err := service.DeleteAccount(context.Background(), id); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if repository.deletedID != id {
		t.Fatalf("deleted ID = %q, want %q", repository.deletedID, id)
	}
}

func TestVerifyTokenRejectsDeletedAccount(t *testing.T) {
	service := newTestService(&accountRepositoryStub{})

	_, err := service.VerifyToken(context.Background(), "validly-signed-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("VerifyToken() error = %v, want %v", err, domain.ErrInvalidToken)
	}
}
