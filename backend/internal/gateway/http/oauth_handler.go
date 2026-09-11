package http

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

type OAuthStartRequest struct {
	RedirectURI   string `json:"redirect_uri"`
	CreateAccount bool   `json:"create_account"`
}
type OAuthStartResponse struct {
	State            string `json:"state"`
	AuthorizationURL string `json:"authorization_url"`
	ExpiresIn        int64  `json:"expires_in"`
}
type OAuthFinishRequest struct {
	RedirectURI   string `json:"redirect_uri"`
	CreateAccount bool   `json:"create_account"`
	State         string `json:"state"`
	Code          string `json:"code"`
}

func validCodeProvider(provider string) bool { return provider == "discord" || provider == "twitter" }

func validOAuthRedirect(c echo.Context, provider, value string) bool {
	uri, err := url.Parse(value)
	return err == nil && uri.User == nil && uri.RawQuery == "" && uri.Fragment == "" &&
		uri.Path == "/auth/callback/"+provider &&
		(uri.Scheme+"://"+uri.Host) == c.Request().Header.Get(echo.HeaderOrigin)
}

// startOAuth godoc
// @Summary Start Discord or X OAuth login/registration
// @Tags Auth
// @Accept json
// @Produce json
// @Param provider path string true "discord or twitter"
// @Param request body OAuthStartRequest true "OAuth transaction"
// @Success 200 {object} OAuthStartResponse
// @Failure 400,403,412,503 {object} ErrorResponse
// @Router /api/v1/auth/oauth/{provider}/start [post]
func (h *Handler) startOAuth(c echo.Context) error {
	provider := c.Param("provider")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	var request OAuthStartRequest
	if !validCodeProvider(provider) || c.Bind(&request) != nil || !validOAuthRedirect(c, provider, request.RedirectURI) {
		return c.JSON(http.StatusBadRequest, errorBody("invalid OAuth redirect URI or provider"))
	}
	response, err := h.auth.StartOAuth(grpcContext(c), &bookstorev1.StartOAuthRequest{
		Provider: provider, RedirectUri: request.RedirectURI, CreateAccount: request.CreateAccount,
	})
	if err != nil {
		return providerErrorResponse(c, provider, err)
	}
	h.setProviderStateCookie(c, provider, request.CreateAccount, response.GetState())
	return c.JSON(http.StatusOK, OAuthStartResponse{
		State: response.GetState(), AuthorizationURL: response.GetAuthorizationUrl(), ExpiresIn: response.GetExpiresIn(),
	})
}

// finishOAuth godoc
// @Summary Complete Discord or X OAuth login/registration
// @Description Exchanges a one-use code/state. Returns a short-lived access token and rotates the HttpOnly refresh cookie. On failure restart OAuth; do not replay the authorization code.
// @Tags Auth
// @Accept json
// @Produce json
// @Param provider path string true "discord or twitter"
// @Param request body OAuthFinishRequest true "OAuth callback"
// @Success 200 {object} AuthResponse
// @Failure 400,401,403,409,412,503 {object} ErrorResponse
// @Router /api/v1/auth/oauth/{provider}/finish [post]
func (h *Handler) finishOAuth(c echo.Context) error {
	provider := c.Param("provider")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	if !h.isTrustedOrigin(c) {
		return c.JSON(http.StatusForbidden, errorBody("untrusted request origin"))
	}
	var request OAuthFinishRequest
	if !validCodeProvider(provider) || c.Bind(&request) != nil || !validOAuthRedirect(c, provider, request.RedirectURI) {
		return c.JSON(http.StatusBadRequest, errorBody("invalid OAuth redirect URI or provider"))
	}
	if !h.validProviderState(c, provider, request.CreateAccount, request.State) {
		return providerError(c, http.StatusForbidden, provider, "invalid_oauth_state", "external login state is invalid or expired", false)
	}
	// The database transaction also atomically consumes state; clearing a cookie alone
	// would not prevent parallel replay.
	h.clearProviderStateCookie(c, provider)
	response, err := h.auth.FinishOAuth(grpcContext(c), &bookstorev1.FinishOAuthRequest{
		Provider: provider, RedirectUri: request.RedirectURI, CreateAccount: request.CreateAccount, State: request.State, Code: request.Code,
	})
	if err != nil {
		return providerErrorResponse(c, provider, err)
	}
	h.setRefreshCookie(c, response.GetRefreshToken(), response.GetRefreshExpiresIn())
	return c.JSON(http.StatusOK, authJSON(response))
}
