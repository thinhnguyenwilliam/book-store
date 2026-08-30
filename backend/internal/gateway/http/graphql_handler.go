package http

import (
	"strings"

	"github.com/labstack/echo/v4"
	bookstorev1 "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	gatewaygraphql "github.com/thinhnguyenwilliam/book-store/backend/internal/gateway/graphql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) graphQLRequest(c echo.Context) error {
	principal, err := h.optionalPrincipal(c)
	if err != nil {
		return errorResponse(c, err)
	}
	request := c.Request()
	request = request.WithContext(gatewaygraphql.ContextWithPrincipal(grpcContext(c), gatewaygraphql.Principal{
		UserID: principal.UserID, Email: principal.Email, Roles: principal.Roles,
	}))
	h.graphQL.ServeHTTP(c.Response(), request)
	return nil
}

func (h *Handler) optionalPrincipal(c echo.Context) (Principal, error) {
	header := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))
	if header == "" {
		return Principal{}, nil
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Principal{}, status.Error(codes.Unauthenticated, "missing or invalid bearer token")
	}
	claims, err := h.auth.VerifyToken(grpcContext(c), &bookstorev1.VerifyTokenRequest{AccessToken: parts[1]})
	if err != nil {
		return Principal{}, err
	}
	return Principal{UserID: claims.GetUserId(), Email: claims.GetEmail(), Roles: claims.GetRoles()}, nil
}
