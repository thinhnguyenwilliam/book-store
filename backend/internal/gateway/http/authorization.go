package http

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	pb "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
)

func hasPermission(principal Principal, permission string) bool {
	return slices.Contains(principal.Permissions, permission)
}

func RequirePermission(permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !hasPermission(principalFromContext(c), permission) {
				return c.JSON(http.StatusForbidden, errorBody("insufficient permissions"))
			}
			return next(c)
		}
	}
}

func bearerToken(c echo.Context) string {
	parts := strings.Fields(c.Request().Header.Get(echo.HeaderAuthorization))
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

type AccessResponse struct {
	AccountID   string   `json:"account_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type RoleResponse struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	System      bool     `json:"system"`
	Permissions []string `json:"permissions"`
}

type PermissionResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

type AuthorizationCatalogResponse struct {
	Roles       []RoleResponse       `json:"roles"`
	Permissions []PermissionResponse `json:"permissions"`
}

type AssignRolesRequest struct {
	Roles []string `json:"roles"`
}

type AuthorizationAuditResponse struct {
	ID        int64  `json:"id"`
	ActorID   string `json:"actor_id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Before    string `json:"before"`
	After     string `json:"after"`
	TraceID   string `json:"trace_id"`
	CreatedAt string `json:"created_at"`
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// getAccess godoc
// @Summary Get current account permissions
// @Tags Authorization
// @Security BearerAuth
// @Produce json
// @Success 200 {object} AccessResponse
// @Router /api/v1/auth/me/permissions [get]
func (h *Handler) getAccess(c echo.Context) error {
	response, err := h.auth.GetAccess(grpcContext(c), &pb.GetAccessRequest{AccessToken: bearerToken(c), AccountId: c.Param("id")})
	if err != nil {
		return errorResponse(c, err)
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, AccessResponse{
		AccountID:   response.GetAccountId(),
		Roles:       stringsOrEmpty(response.GetRoles()),
		Permissions: stringsOrEmpty(response.GetPermissions()),
	})
}

// authorizationCatalog godoc
// @Summary List roles and permission catalog
// @Tags Authorization
// @Security BearerAuth
// @Success 200 {object} AuthorizationCatalogResponse
// @Router /api/v1/admin/roles [get]
func (h *Handler) authorizationCatalog(c echo.Context) error {
	response, err := h.auth.GetAuthorizationCatalog(grpcContext(c), &pb.AuthorizationRequest{AccessToken: bearerToken(c)})
	if err != nil {
		return errorResponse(c, err)
	}
	value := AuthorizationCatalogResponse{Roles: []RoleResponse{}, Permissions: []PermissionResponse{}}
	for _, r := range response.GetRoles() {
		value.Roles = append(value.Roles, RoleResponse{Code: r.GetCode(), Name: r.GetName(), Description: r.GetDescription(), System: r.GetSystem(), Permissions: stringsOrEmpty(r.GetPermissions())})
	}
	for _, p := range response.GetPermissions() {
		value.Permissions = append(value.Permissions, PermissionResponse{Code: p.GetCode(), Name: p.GetName(), Description: p.GetDescription(), Group: p.GetGroup()})
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, value)
}

// saveRole godoc
// @Summary Create or update a custom role (system roles are immutable)
// @Tags Authorization
// @Security BearerAuth
// @Accept json
// @Param request body RoleResponse true "Role permissions"
// @Success 204
// @Router /api/v1/admin/roles [post]
// @Router /api/v1/admin/roles/{code} [put]
func (h *Handler) saveRole(c echo.Context) error {
	var value RoleResponse
	if err := c.Bind(&value); err != nil {
		return c.JSON(http.StatusBadRequest, errorBody("invalid role"))
	}
	create := c.Request().Method == http.MethodPost
	if !create {
		value.Code = c.Param("code")
	}
	_, err := h.auth.SaveRole(grpcContext(c), &pb.SaveRoleRequest{AccessToken: bearerToken(c), Create: create, Role: &pb.Role{Code: value.Code, Name: value.Name, Description: value.Description, Permissions: value.Permissions}})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// deleteRole godoc
// @Summary Delete a custom role that is not assigned to any account
// @Tags Authorization
// @Security BearerAuth
// @Success 204
// @Failure 400,403,404,412 {object} ErrorResponse
// @Router /api/v1/admin/roles/{code} [delete]
func (h *Handler) deleteRole(c echo.Context) error {
	_, err := h.auth.DeleteRole(grpcContext(c), &pb.DeleteRoleRequest{AccessToken: bearerToken(c), Code: c.Param("code")})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// assignRoles godoc
// @Summary Replace account roles
// @Tags Authorization
// @Security BearerAuth
// @Accept json
// @Param id path string true "Account ID"
// @Param request body AssignRolesRequest true "Role codes"
// @Success 204
// @Router /api/v1/admin/accounts/{id}/roles [put]
func (h *Handler) assignRoles(c echo.Context) error {
	var value AssignRolesRequest
	if err := c.Bind(&value); err != nil {
		return c.JSON(http.StatusBadRequest, errorBody("invalid roles"))
	}
	_, err := h.auth.AssignRoles(grpcContext(c), &pb.AssignRolesRequest{AccessToken: bearerToken(c), AccountId: c.Param("id"), Roles: value.Roles})
	if err != nil {
		return errorResponse(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// authorizationAudit godoc
// @Summary List authorization changes (50 entries, descending ID)
// @Tags Authorization
// @Security BearerAuth
// @Param before_id query int false "Exclusive cursor"
// @Success 200 {array} AuthorizationAuditResponse
// @Router /api/v1/admin/authorization/audit [get]
func (h *Handler) authorizationAudit(c echo.Context) error {
	var before int64
	if raw := c.QueryParam("before_id"); raw != "" {
		var err error
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, errorBody("invalid cursor"))
		}
	}
	response, err := h.auth.ListAuthorizationAudit(grpcContext(c), &pb.AuthorizationAuditRequest{AccessToken: bearerToken(c), BeforeId: before})
	if err != nil {
		return errorResponse(c, err)
	}
	values := []AuthorizationAuditResponse{}
	for _, v := range response.GetEntries() {
		values = append(values, AuthorizationAuditResponse{ID: v.GetId(), ActorID: v.GetActorId(), Action: v.GetAction(), Target: v.GetTarget(), Before: v.GetBefore(), After: v.GetAfter(), TraceID: v.GetTraceId(), CreatedAt: v.GetCreatedAt()})
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, values)
}
