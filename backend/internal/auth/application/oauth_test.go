package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type oauthStoreStub struct{ tx OAuthTransaction }

func (r *oauthStoreStub) SaveOAuthTransaction(_ context.Context, tx OAuthTransaction) error {
	r.tx = tx
	return nil
}
func (r *oauthStoreStub) ConsumeOAuthTransaction(_ context.Context, hash, provider, uri string, create bool, now time.Time) (OAuthTransaction, error) {
	tx := r.tx
	if tx.StateHash != hash || tx.Provider != provider || tx.RedirectURI != uri || tx.CreateAccount != create || !tx.ExpiresAt.After(now) {
		return OAuthTransaction{}, domain.ErrOAuthState
	}
	r.tx = OAuthTransaction{}
	return tx, nil
}

type oauthProviderStub struct{ calls int }

func (*oauthProviderStub) AuthorizationURL(_, _, _ string) (string, error) {
	return "https://provider.example/authorize", nil
}
func (p *oauthProviderStub) Exchange(context.Context, string, string, string) (VerifiedIdentity, error) {
	p.calls++
	return VerifiedIdentity{Provider: "discord", Subject: "123", Email: "reader@example.com", EmailVerified: true}, nil
}
func TestOAuthStateBindingReplayAndCustomerRole(t *testing.T) {
	repo, store, provider := &accountRepositoryStub{}, &oauthStoreStub{}, &oauthProviderStub{}
	s := newTestService(repo)
	s.SetOAuthProviders(store, map[string]OAuthProvider{"discord": provider})
	start, err := s.StartOAuth(context.Background(), "discord", "https://store.example/callback", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(start.State) != 43 || store.tx.StateHash == start.State {
		t.Fatal("state must be random and hashed at rest")
	}
	for _, tc := range []struct {
		uri    string
		create bool
	}{{"https://evil.example", true}, {"https://store.example/callback", false}} {
		_, err := s.FinishOAuth(context.Background(), "discord", "code", start.State, tc.uri, tc.create)
		if !errors.Is(err, domain.ErrOAuthState) {
			t.Fatalf("invalid binding accepted: %v", err)
		}
	}
	_, err = s.FinishOAuth(context.Background(), "discord", "code", start.State, "https://store.example/callback", true)
	if err != nil {
		t.Fatal(err)
	}
	if repo.created == nil || len(repo.created.Roles) != 1 || repo.created.Roles[0] != "customer" {
		t.Fatal("new identity must be customer")
	}
	_, err = s.FinishOAuth(context.Background(), "discord", "code", start.State, "https://store.example/callback", true)
	if !errors.Is(err, domain.ErrOAuthState) || provider.calls != 1 {
		t.Fatal("replay reached provider")
	}
}
func TestOAuthExpiryAndNoAutomaticEmailLink(t *testing.T) {
	repo := &accountRepositoryStub{account: &domain.Account{ID: "existing", Email: "reader@example.com", Roles: []string{"admin"}}}
	store, provider := &oauthStoreStub{}, &oauthProviderStub{}
	s := newTestService(repo)
	s.SetOAuthProviders(store, map[string]OAuthProvider{"discord": provider})
	start, err := s.StartOAuth(context.Background(), "discord", "uri", true)
	if err != nil {
		t.Fatal(err)
	}
	store.tx.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := s.FinishOAuth(context.Background(), "discord", "code", start.State, "uri", true); !errors.Is(err, domain.ErrOAuthState) {
		t.Fatal("expired state accepted")
	}
	start, err = s.StartOAuth(context.Background(), "discord", "uri", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.FinishOAuth(context.Background(), "discord", "code", start.State, "uri", true); !errors.Is(err, domain.ErrIdentityConflict) {
		t.Fatal("email collision must not link admin account")
	}
}
