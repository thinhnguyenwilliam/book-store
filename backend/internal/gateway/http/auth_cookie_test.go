package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRefreshCookieUsesRestrictedSecurityAttributes(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(request, recorder)
	handler := &Handler{refreshCookie: RefreshCookieConfig{
		Name:     "bookstore_refresh",
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}}

	handler.setRefreshCookie(ctx, "secret-refresh-token", 3600)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookie security attributes: %+v", cookie)
	}
	if cookie.Path != refreshCookiePath || cookie.MaxAge != 3600 {
		t.Fatalf("unexpected cookie scope or lifetime: %+v", cookie)
	}
}

func TestTrustedOriginAllowsConfiguredOriginAndNonBrowserClient(t *testing.T) {
	handler := &Handler{trustedOrigins: map[string]struct{}{
		"http://localhost:5173": {},
	}}
	e := echo.New()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.Header.Set(echo.HeaderOrigin, "http://localhost:5173")
	if !handler.isTrustedOrigin(e.NewContext(request, httptest.NewRecorder())) {
		t.Fatal("configured storefront origin should be trusted")
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.Header.Set(echo.HeaderOrigin, "https://evil.example")
	if handler.isTrustedOrigin(e.NewContext(request, httptest.NewRecorder())) {
		t.Fatal("unconfigured browser origin should be rejected")
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	if !handler.isTrustedOrigin(e.NewContext(request, httptest.NewRecorder())) {
		t.Fatal("non-browser client without Origin should be allowed")
	}
}
