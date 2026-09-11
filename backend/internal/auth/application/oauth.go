package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type OAuthTransaction struct {
	StateHash     string
	Provider      string
	RedirectURI   string
	Verifier      string
	CreateAccount bool
	ExpiresAt     time.Time
}

type OAuthTransactionStore interface {
	SaveOAuthTransaction(context.Context, OAuthTransaction) error
	ConsumeOAuthTransaction(context.Context, string, string, string, bool, time.Time) (OAuthTransaction, error)
}

type OAuthProvider interface {
	AuthorizationURL(state, verifier, redirectURI string) (string, error)
	Exchange(context.Context, string, string, string) (VerifiedIdentity, error)
}

type OAuthStart struct {
	State            string
	AuthorizationURL string
	ExpiresIn        int64
}

func (s *Service) SetOAuthProviders(store OAuthTransactionStore, providers map[string]OAuthProvider) {
	s.oauthTransactions, s.oauthProviders = store, providers
}

func (s *Service) StartOAuth(ctx context.Context, provider, redirectURI string, createAccount bool) (OAuthStart, error) {
	p := s.oauthProviders[provider]
	if p == nil || s.oauthTransactions == nil {
		return OAuthStart{}, domain.ErrIdentityUnavailable
	}
	state, err := randomOAuthValue()
	if err != nil {
		return OAuthStart{}, err
	}
	verifier, err := randomOAuthValue()
	if err != nil {
		return OAuthStart{}, err
	}
	authURL, err := p.AuthorizationURL(state, verifier, redirectURI)
	if err != nil {
		return OAuthStart{}, err
	}
	tx := OAuthTransaction{StateHash: hashOAuthState(state), Provider: provider, RedirectURI: redirectURI,
		Verifier: verifier, CreateAccount: createAccount, ExpiresAt: s.now().UTC().Add(10 * time.Minute)}
	if err := s.oauthTransactions.SaveOAuthTransaction(ctx, tx); err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{State: state, AuthorizationURL: authURL, ExpiresIn: 600}, nil
}

func (s *Service) FinishOAuth(ctx context.Context, provider, code, state, redirectURI string, createAccount bool) (AuthResult, error) {
	p := s.oauthProviders[provider]
	if p == nil || s.oauthTransactions == nil {
		return AuthResult{}, domain.ErrIdentityUnavailable
	}
	if len(state) != 43 || code == "" || len(code) > 4096 {
		return AuthResult{}, domain.ErrOAuthState
	}
	// Atomic consume before exchange prevents parallel/replayed callback requests.
	tx, err := s.oauthTransactions.ConsumeOAuthTransaction(ctx, hashOAuthState(state), provider, redirectURI, createAccount, s.now().UTC())
	if err != nil {
		return AuthResult{}, err
	}
	identity, err := p.Exchange(ctx, code, tx.Verifier, tx.RedirectURI)
	if err != nil {
		return AuthResult{}, err
	}
	return s.loginVerifiedIdentity(ctx, provider, identity, tx.CreateAccount)
}

func randomOAuthValue() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hashOAuthState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}
