package grpc

import (
	"context"

	pb "github.com/thinhnguyenwilliam/book-store/backend/gen/bookstore/v1"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

func (h *Handler) GetAccess(ctx context.Context, req *pb.GetAccessRequest) (*pb.AccessResponse, error) {
	value, err := h.service.AccountAccess(ctx, req.GetAccessToken(), req.GetAccountId())
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.AccessResponse{AccountId: value.AccountID, Roles: value.Roles, Permissions: value.Permissions}, nil
}
func (h *Handler) GetAuthorizationCatalog(ctx context.Context, req *pb.AuthorizationRequest) (*pb.AuthorizationCatalogResponse, error) {
	roles, permissions, err := h.service.AuthorizationCatalog(ctx, req.GetAccessToken())
	if err != nil {
		return nil, mapError(err)
	}
	response := &pb.AuthorizationCatalogResponse{}
	for _, r := range roles {
		response.Roles = append(response.Roles, &pb.Role{Code: r.Code, Name: r.Name, Description: r.Description, System: r.System, Permissions: r.Permissions})
	}
	for _, p := range permissions {
		response.Permissions = append(response.Permissions, &pb.Permission{Code: p.Code, Name: p.Name, Description: p.Description, Group: p.Group})
	}
	return response, nil
}
func (h *Handler) SaveRole(ctx context.Context, req *pb.SaveRoleRequest) (*pb.AuthorizationEmpty, error) {
	r := req.GetRole()
	err := h.service.SaveRole(ctx, req.GetAccessToken(), domain.Role{Code: r.GetCode(), Name: r.GetName(), Description: r.GetDescription(), Permissions: r.GetPermissions()}, req.GetCreate())
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.AuthorizationEmpty{}, nil
}
func (h *Handler) DeleteRole(ctx context.Context, req *pb.DeleteRoleRequest) (*pb.AuthorizationEmpty, error) {
	if err := h.service.DeleteRole(ctx, req.GetAccessToken(), req.GetCode()); err != nil {
		return nil, mapError(err)
	}
	return &pb.AuthorizationEmpty{}, nil
}
func (h *Handler) AssignRoles(ctx context.Context, req *pb.AssignRolesRequest) (*pb.AuthorizationEmpty, error) {
	if err := h.service.AssignRoles(ctx, req.GetAccessToken(), req.GetAccountId(), req.GetRoles()); err != nil {
		return nil, mapError(err)
	}
	return &pb.AuthorizationEmpty{}, nil
}
func (h *Handler) ListAuthorizationAudit(ctx context.Context, req *pb.AuthorizationAuditRequest) (*pb.AuthorizationAuditResponse, error) {
	values, err := h.service.AuthorizationAudit(ctx, req.GetAccessToken(), req.GetBeforeId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &pb.AuthorizationAuditResponse{}
	for _, v := range values {
		response.Entries = append(response.Entries, &pb.AuthorizationAudit{Id: v.ID, ActorId: v.ActorID, Action: v.Action, Target: v.Target, Before: v.Before, After: v.After, TraceId: v.TraceID, CreatedAt: v.CreatedAt})
	}
	return response, nil
}
