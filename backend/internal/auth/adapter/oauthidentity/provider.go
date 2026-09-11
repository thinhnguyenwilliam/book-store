package oauthidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURIs []string
}

type Provider struct {
	provider     string
	config       Config
	authorizeURL string
	tokenURL     string
	profileURL   string
	scopes       string
	client       *http.Client
}

func New(provider string, cfg Config) *Provider {
	p := &Provider{provider: provider, config: cfg, client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	switch provider {
	case domain.IdentityProviderDiscord:
		p.authorizeURL, p.tokenURL, p.profileURL = "https://discord.com/oauth2/authorize", "https://discord.com/api/oauth2/token", "https://discord.com/api/v10/users/@me"
		p.scopes = "identify email"
	case domain.IdentityProviderTwitter:
		p.authorizeURL, p.tokenURL, p.profileURL = "https://x.com/i/oauth2/authorize", "https://api.x.com/2/oauth2/token", "https://api.x.com/2/users/me?user.fields=confirmed_email"
		p.scopes = "tweet.read users.read users.email"
	}
	return p
}

func (p *Provider) allowedRedirect(uri string) bool {
	for _, allowed := range p.config.RedirectURIs {
		if uri == allowed {
			return true
		}
	}
	return false
}

func (p *Provider) AuthorizationURL(state, verifier, redirectURI string) (string, error) {
	if p.config.ClientID == "" || p.config.ClientSecret == "" || p.authorizeURL == "" {
		return "", domain.ErrIdentityUnavailable
	}
	if !p.allowedRedirect(redirectURI) {
		return "", domain.ErrInvalidInput
	}
	values := url.Values{"response_type": {"code"}, "client_id": {p.config.ClientID},
		"redirect_uri": {redirectURI}, "scope": {p.scopes}, "state": {state}}
	if p.provider == domain.IdentityProviderTwitter {
		digest := sha256.Sum256([]byte(verifier))
		values.Set("code_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
		values.Set("code_challenge_method", "S256")
	}
	return p.authorizeURL + "?" + values.Encode(), nil
}

func (p *Provider) Exchange(ctx context.Context, code, verifier, redirectURI string) (application.VerifiedIdentity, error) {
	if _, err := p.AuthorizationURL("", verifier, redirectURI); err != nil {
		return application.VerifiedIdentity{}, err
	}
	values := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}}
	if p.provider == domain.IdentityProviderTwitter {
		values.Set("code_verifier", verifier)
	} else {
		values.Set("client_id", p.config.ClientID)
		values.Set("client_secret", p.config.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return application.VerifiedIdentity{}, domain.ErrIdentityProvider
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.provider == domain.IdentityProviderTwitter {
		req.SetBasicAuth(p.config.ClientID, p.config.ClientSecret)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := p.request(req, &token); err != nil {
		return application.VerifiedIdentity{}, err
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "bearer") {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, p.profileURL, nil)
	if err != nil {
		return application.VerifiedIdentity{}, domain.ErrIdentityProvider
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	// Provider tokens are used only here: never persisted or returned to browsers.
	identity := application.VerifiedIdentity{Provider: p.provider, EmailAuthoritative: false}
	if p.provider == domain.IdentityProviderDiscord {
		var profile struct {
			ID         string `json:"id"`
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
			Email      string `json:"email"`
			Verified   bool   `json:"verified"`
		}
		if err := p.request(req, &profile); err != nil {
			return identity, err
		}
		identity.Subject, identity.Email, identity.DisplayName = profile.ID, profile.Email, profile.GlobalName
		if identity.DisplayName == "" {
			identity.DisplayName = profile.Username
		}
		identity.EmailVerified = profile.Verified
	} else {
		var profile struct {
			Data struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"confirmed_email"`
			} `json:"data"`
		}
		if err := p.request(req, &profile); err != nil {
			return identity, err
		}
		identity.Subject, identity.Email, identity.DisplayName = profile.Data.ID, profile.Data.Email, profile.Data.Name
		identity.EmailVerified = profile.Data.Email != ""
	}
	if identity.Subject == "" {
		return identity, domain.ErrInvalidIdentity
	}
	if strings.TrimSpace(identity.Email) == "" || !identity.EmailVerified {
		return identity, domain.ErrIdentityEmailRequired
	}
	return identity, nil
}

func (p *Provider) request(req *http.Request, result any) error {
	response, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return domain.ErrIdentityProvider
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return domain.ErrIdentityProvider
	}
	if response.StatusCode != http.StatusOK {
		return domain.ErrInvalidIdentity
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result); err != nil {
		return domain.ErrInvalidIdentity
	}
	return nil
}
