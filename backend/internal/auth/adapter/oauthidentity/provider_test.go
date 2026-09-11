package oauthidentity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

func TestProviderCodeExchange(t *testing.T) {
	for _, provider := range []string{"discord", "twitter"} {
		t.Run(provider, func(t *testing.T) {
			callback := "http://localhost:5173/auth/callback/" + provider
			p := New(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURIs: []string{callback}})
			authURL, err := p.AuthorizationURL("state", "verifier", callback)
			if err != nil {
				t.Fatal(err)
			}
			parsed, _ := url.Parse(authURL)
			if parsed.Query().Get("state") != "state" || parsed.Query().Get("redirect_uri") != callback {
				t.Fatal("unbound authorization")
			}
			if provider == "twitter" && (parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("code_challenge") == "verifier") {
				t.Fatal("PKCE must use S256")
			}
			if _, err := p.AuthorizationURL("state", "verifier", "https://evil.example"); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatal("redirect not rejected")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/token" {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if r.Form.Get("code") != "code" || r.Form.Get("redirect_uri") != callback {
						t.Error("incorrect exchange")
					}
					if provider == "twitter" {
						id, secret, ok := r.BasicAuth()
						if !ok || id != "client" || secret != "secret" || r.Form.Get("code_verifier") != "verifier" {
							t.Error("missing PKCE/client authentication")
						}
					} else if r.Form.Get("client_secret") != "secret" {
						t.Error("missing Discord client secret")
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "provider-token", "token_type": "Bearer"})
					return
				}
				if r.Header.Get("Authorization") != "Bearer provider-token" {
					t.Error("missing user bearer")
				}
				if provider == "discord" {
					_, _ = w.Write([]byte(`{"id":"123","email":"reader@example.com","verified":true,"global_name":"Reader"}`))
				} else {
					_, _ = w.Write([]byte(`{"data":{"id":"123","confirmed_email":"reader@example.com","name":"Reader"}}`))
				}
			}))
			defer server.Close()
			p.tokenURL, p.profileURL = server.URL+"/token", server.URL+"/me"
			identity, err := p.Exchange(context.Background(), "code", "verifier", callback)
			if err != nil {
				t.Fatal(err)
			}
			if identity.Subject != "123" || !identity.EmailVerified || identity.EmailAuthoritative {
				t.Fatalf("unsafe identity: %+v", identity)
			}
		})
	}
}

func TestProviderRejectsMissingVerifiedEmail(t *testing.T) {
	for _, provider := range []string{"discord", "twitter"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					_, _ = w.Write([]byte(`{"access_token":"x","token_type":"bearer"}`))
					return
				}
				if provider == "discord" {
					_, _ = w.Write([]byte(`{"id":"123","email":"a@example.com","verified":false}`))
				} else {
					_, _ = w.Write([]byte(`{"data":{"id":"123","name":"Reader"}}`))
				}
			}))
			defer server.Close()
			p := New(provider, Config{ClientID: "c", ClientSecret: "s", RedirectURIs: []string{"https://store.example/callback"}})
			p.tokenURL, p.profileURL = server.URL+"/token", server.URL+"/me"
			_, err := p.Exchange(context.Background(), "code", "verifier", "https://store.example/callback")
			if !errors.Is(err, domain.ErrIdentityEmailRequired) {
				t.Fatalf("got %v", err)
			}
		})
	}
}
