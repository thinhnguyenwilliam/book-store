package http

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

const principalContextKey = "principal"

type Principal struct {
	UserID string
	Email  string
	Roles  []string
}

func (h *Handler) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get(echo.HeaderAuthorization)
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.JSON(http.StatusUnauthorized, errorBody("missing or invalid bearer token"))
		}

		ctx := grpcContext(c)
		claims, err := h.auth.VerifyToken(ctx, &bookstorev1.VerifyTokenRequest{AccessToken: parts[1]})
		if err != nil {
			return errorResponse(c, err)
		}

		c.Set(principalContextKey, Principal{
			UserID: claims.GetUserId(),
			Email:  claims.GetEmail(),
			Roles:  claims.GetRoles(),
		})
		return next(c)
	}
}

func RequireRole(requiredRole string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			principal := principalFromContext(c)
			for _, role := range principal.Roles {
				if role == requiredRole {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, errorBody("insufficient permissions"))
		}
	}
}

func principalFromContext(c echo.Context) Principal {
	principal, _ := c.Get(principalContextKey).(Principal)
	return principal
}
