package http

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

const refreshCookiePath = "/api/v1/auth"

func (h *Handler) setRefreshCookie(c echo.Context, token string, expiresIn int64) {
	maxAge := int(expiresIn)
	c.SetCookie(&http.Cookie{
		Name:     h.refreshCookie.Name,
		Value:    token,
		Path:     refreshCookiePath,
		Expires:  time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.refreshCookie.Secure,
		SameSite: h.refreshCookie.SameSite,
	})
}

func (h *Handler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     h.refreshCookie.Name,
		Value:    "",
		Path:     refreshCookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.refreshCookie.Secure,
		SameSite: h.refreshCookie.SameSite,
	})
}

func (h *Handler) isTrustedOrigin(c echo.Context) bool {
	origin := c.Request().Header.Get(echo.HeaderOrigin)
	if origin == "" {
		return true
	}
	_, trusted := h.trustedOrigins[origin]
	return trusted
}
