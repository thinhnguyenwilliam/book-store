package http

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPermissionNotLegacyRole(t *testing.T) {
	for _, tc := range []struct {
		principal Principal
		status    int
	}{
		{Principal{Roles: []string{"admin"}}, http.StatusForbidden},
		{Principal{Permissions: []string{"books.read"}}, http.StatusForbidden},
		{Principal{Permissions: []string{"books.update"}}, http.StatusNoContent},
	} {
		rec := httptest.NewRecorder()
		ctx := echo.New().NewContext(httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/", nil), rec)
		ctx.Set(principalContextKey, tc.principal)
		err := RequirePermission("books.update")(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })(ctx)
		if err != nil || rec.Code != tc.status {
			t.Fatalf("status=%d error=%v", rec.Code, err)
		}
	}
}
