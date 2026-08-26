package http

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	providerGoogle       = "google"
	providerFacebook     = "facebook"
	providerStateTTL     = 10 * time.Minute
	providerStateBytes   = 32
	providerStateMaxSize = 128
)

// providerState godoc
// @Summary Start an external login transaction
// @Description Issues a short-lived state value and matching HttpOnly cookie for Google or Facebook login CSRF protection.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ProviderStateRequest true "External login transaction"
// @Success 200 {object} ProviderStateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/provider-state [post]
func (h *Handler) providerState(c echo.Context) error {
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	request := ProviderStateRequest{}
	if err := c.Bind(&request); err != nil || !validProvider(request.Provider) {
		return c.JSON(http.StatusBadRequest, errorBody("invalid identity provider"))
	}

	state, err := generateProviderState()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorBody("could not start external login"))
	}
	h.setProviderStateCookie(c, request.Provider, request.CreateAccount, state)
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, ProviderStateResponse{
		State:     state,
		ExpiresIn: int64(providerStateTTL.Seconds()),
	})
}

func (h *Handler) setProviderStateCookie(c echo.Context, provider string, createAccount bool, state string) {
	c.SetCookie(&http.Cookie{
		Name:     h.providerStateCookieName(provider),
		Value:    providerStateCookieValue(createAccount, state),
		Path:     refreshCookiePath,
		Expires:  time.Now().UTC().Add(providerStateTTL),
		MaxAge:   int(providerStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.refreshCookie.Secure,
		SameSite: h.refreshCookie.SameSite,
	})
}

func (h *Handler) clearProviderStateCookie(c echo.Context, provider string) {
	c.SetCookie(&http.Cookie{
		Name:     h.providerStateCookieName(provider),
		Value:    "",
		Path:     refreshCookiePath,
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.refreshCookie.Secure,
		SameSite: h.refreshCookie.SameSite,
	})
}

func (h *Handler) validProviderState(c echo.Context, provider string, createAccount bool, state string) bool {
	state = strings.TrimSpace(state)
	if !validProvider(provider) || state == "" || len(state) > providerStateMaxSize {
		return false
	}
	cookie, err := c.Cookie(h.providerStateCookieName(provider))
	if err != nil {
		return false
	}
	want := providerStateCookieValue(createAccount, state)
	return len(cookie.Value) == len(want) && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(want)) == 1
}

func (h *Handler) providerStateCookieName(provider string) string {
	return h.refreshCookie.Name + "_" + provider + "_state"
}

func providerStateCookieValue(createAccount bool, state string) string {
	return strconv.FormatBool(createAccount) + "." + state
}

func generateProviderState() (string, error) {
	random := make([]byte, providerStateBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func validProvider(provider string) bool {
	return provider == providerGoogle || provider == providerFacebook
}
