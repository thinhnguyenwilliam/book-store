package facebookidentity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestVerifyValidFacebookUserToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v25.0/debug_token":
			if request.Header.Get("Authorization") != "Bearer app-id|app-secret" {
				t.Fatalf("unexpected app authorization header")
			}
			if request.URL.Query().Get("input_token") != "user-token" {
				t.Fatalf("input_token was not sent")
			}
			return jsonResponse(http.StatusOK, `{"data":{"app_id":"app-id","type":"USER","is_valid":true,"user_id":"facebook-user","expires_at":1800003600,"data_access_expires_at":1800007200}}`), nil
		case "/v25.0/me":
			if request.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatalf("unexpected user authorization header")
			}
			if request.URL.Query().Get("fields") != "id,name,email" {
				t.Fatalf("profile fields were not requested")
			}
			if request.URL.Query().Get("appsecret_proof") == "" {
				t.Fatalf("appsecret_proof was not sent")
			}
			return jsonResponse(http.StatusOK, `{"id":"facebook-user","name":"Reader Nguyen","email":"Reader@Example.com"}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})}

	verifier := NewVerifier("app-id", "app-secret", "v25.0")
	verifier.baseURL = "https://graph.facebook.test"
	verifier.client = client
	verifier.now = func() time.Time { return now }

	identity, err := verifier.Verify(context.Background(), "user-token", "")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if identity.Provider != domain.IdentityProviderFacebook || identity.Subject != "facebook-user" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if identity.Email != "reader@example.com" || !identity.EmailVerified || !identity.EmailAuthoritative {
		t.Fatalf("unexpected email identity: %+v", identity)
	}
}

func TestVerifyRejectsTokenForAnotherApp(t *testing.T) {
	verifier := NewVerifier("app-id", "app-secret", "v25.0")
	verifier.baseURL = "https://graph.facebook.test"
	verifier.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"app_id":"another-app","type":"USER","is_valid":true,"user_id":"facebook-user"}}`), nil
	})}

	_, err := verifier.Verify(context.Background(), "user-token", "")
	if !errors.Is(err, domain.ErrInvalidIdentity) {
		t.Fatalf("Verify() error = %v, want %v", err, domain.ErrInvalidIdentity)
	}
}

func TestVerifyReportsGraphAPIFailure(t *testing.T) {
	verifier := NewVerifier("app-id", "app-secret", "v25.0")
	verifier.baseURL = "https://graph.facebook.test"
	verifier.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, `{}`), nil
	})}

	_, err := verifier.Verify(context.Background(), "user-token", "")
	if !errors.Is(err, domain.ErrIdentityProvider) {
		t.Fatalf("Verify() error = %v, want %v", err, domain.ErrIdentityProvider)
	}
}

func TestVerifyRequiresServerCredentials(t *testing.T) {
	verifier := NewVerifier("app-id", "", "v25.0")

	_, err := verifier.Verify(context.Background(), "user-token", "")
	if !errors.Is(err, domain.ErrIdentityUnavailable) {
		t.Fatalf("Verify() error = %v, want %v", err, domain.ErrIdentityUnavailable)
	}
}
