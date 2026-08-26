package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestProviderStateUsesBoundHttpOnlyCookie(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/auth/provider-state",
		strings.NewReader(`{"provider":"google","create_account":true}`),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderOrigin, "http://localhost:5173")
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(request, recorder)
	handler := &Handler{
		refreshCookie: RefreshCookieConfig{
			Name:     "bookstore_refresh",
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		},
		trustedOrigins: map[string]struct{}{"http://localhost:5173": {}},
	}

	if err := handler.providerState(ctx); err != nil {
		t.Fatalf("providerState() error = %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response ProviderStateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State == "" || response.ExpiresIn != int64(providerStateTTL.Seconds()) {
		t.Fatalf("unexpected provider state response: %+v", response)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "bookstore_refresh_google_state" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatalf("unexpected state cookie: %+v", cookie)
	}

	validationRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	validationRequest.AddCookie(cookie)
	validationContext := e.NewContext(validationRequest, httptest.NewRecorder())
	if !handler.validProviderState(validationContext, providerGoogle, true, response.State) {
		t.Fatal("issued state should validate for its provider and account intent")
	}
	if handler.validProviderState(validationContext, providerGoogle, false, response.State) {
		t.Fatal("state must be bound to create_account intent")
	}
	if handler.validProviderState(validationContext, providerFacebook, true, response.State) {
		t.Fatal("state must be bound to its provider")
	}
}

func TestProviderStateRejectsUnknownProvider(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/auth/provider-state",
		strings.NewReader(`{"provider":"unknown","create_account":false}`),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recorder := httptest.NewRecorder()
	handler := &Handler{refreshCookie: RefreshCookieConfig{Name: "bookstore_refresh"}}

	if err := handler.providerState(e.NewContext(request, recorder)); err != nil {
		t.Fatalf("providerState() error = %v", err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
