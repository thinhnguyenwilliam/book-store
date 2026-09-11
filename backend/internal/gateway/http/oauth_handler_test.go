package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestOAuthRejectsUnboundRequestsBeforeRPC(t *testing.T) {
	for _, tc := range []struct {
		name, origin, provider, body string
		finish                       bool
		status                       int
	}{
		{"untrusted origin", "https://evil.example", "discord", `{}`, false, http.StatusForbidden},
		{"cross-origin redirect", "http://localhost:5173", "discord", `{"redirect_uri":"https://evil.example/auth/callback/discord"}`, false, http.StatusBadRequest},
		{"wrong callback", "http://localhost:5173", "discord", `{"redirect_uri":"http://localhost:5173/auth/callback/twitter"}`, false, http.StatusBadRequest},
		{"callback query", "http://localhost:5173", "discord", `{"redirect_uri":"http://localhost:5173/auth/callback/discord?next=evil"}`, false, http.StatusBadRequest},
		{"missing state cookie", "http://localhost:5173", "discord", `{"redirect_uri":"http://localhost:5173/auth/callback/discord","state":"attacker","code":"code"}`, true, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderOrigin, tc.origin)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			ctx := echo.New().NewContext(req, rec)
			ctx.SetParamNames("provider")
			ctx.SetParamValues(tc.provider)
			// A nil auth client ensures rejected requests never reach Auth Service.
			h := &Handler{trustedOrigins: map[string]struct{}{"http://localhost:5173": {}}, refreshCookie: RefreshCookieConfig{Name: "bookstore_refresh"}}
			var err error
			if tc.finish {
				err = h.finishOAuth(ctx)
			} else {
				err = h.startOAuth(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if rec.Header().Get(echo.HeaderCacheControl) != "no-store" {
				t.Fatal("OAuth responses must not be cached")
			}
		})
	}
}
