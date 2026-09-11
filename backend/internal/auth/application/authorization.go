package application

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type AuthorizationRepository interface {
	Access(context.Context, string) (domain.Access, error)
	ListRoles(context.Context) ([]domain.Role, error)
	ListPermissions(context.Context) ([]domain.Permission, error)
	SaveRole(context.Context, string, domain.Role, bool) error
	DeleteRole(context.Context, string, string) error
	AssignRoles(context.Context, string, string, []string) error
	ListAuthorizationAudit(context.Context, int64) ([]domain.AuthorizationAudit, error)
}

func (s *Service) SetAuthorizationRepository(repository AuthorizationRepository) {
	s.authorization = repository
}

func (s *Service) RequirePermission(ctx context.Context, token, permission string) (Claims, error) {
	claims, err := s.VerifyToken(ctx, token)
	if err != nil {
		return Claims{}, err
	}
	if !slices.Contains(claims.Permissions, permission) {
		return Claims{}, domain.ErrForbidden
	}
	return claims, nil
}
func (s *Service) AuthorizationCatalog(ctx context.Context, token string) ([]domain.Role, []domain.Permission, error) {
	if _, err := s.RequirePermission(ctx, token, "roles.read"); err != nil {
		return nil, nil, err
	}
	roles, err := s.authorization.ListRoles(ctx)
	if err != nil {
		return nil, nil, err
	}
	permissions, err := s.authorization.ListPermissions(ctx)
	return roles, permissions, err
}

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func (s *Service) SaveRole(ctx context.Context, token string, role domain.Role, create bool) error {
	claims, err := s.RequirePermission(ctx, token, "roles.manage")
	if err != nil {
		return err
	}
	role.Code = strings.TrimSpace(role.Code)
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	role.Permissions = uniqueSorted(role.Permissions)
	if !roleCodePattern.MatchString(role.Code) || role.Name == "" || len(role.Name) > 120 || len(role.Description) > 1000 || len(role.Permissions) == 0 || len(role.Permissions) > 100 {
		return domain.ErrInvalidInput
	}
	return s.authorization.SaveRole(ctx, claims.UserID, role, create)
}

func (s *Service) DeleteRole(ctx context.Context, token, code string) error {
	claims, err := s.RequirePermission(ctx, token, "roles.manage")
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if !roleCodePattern.MatchString(code) {
		return domain.ErrInvalidInput
	}
	return s.authorization.DeleteRole(ctx, claims.UserID, code)
}
func (s *Service) AccountAccess(ctx context.Context, token, id string) (domain.Access, error) {
	claims, err := s.VerifyToken(ctx, token)
	if err != nil {
		return domain.Access{}, err
	}
	if id == "" || id == claims.UserID {
		return domain.Access{AccountID: claims.UserID, Roles: claims.Roles, Permissions: claims.Permissions}, nil
	}
	if !slices.Contains(claims.Permissions, "roles.read") {
		return domain.Access{}, domain.ErrForbidden
	}
	if _, err = uuid.Parse(id); err != nil {
		return domain.Access{}, domain.ErrInvalidInput
	}
	return s.authorization.Access(ctx, id)
}
func (s *Service) AssignRoles(ctx context.Context, token, id string, roles []string) error {
	claims, err := s.RequirePermission(ctx, token, "users.assign_roles")
	if err != nil {
		return err
	}
	if id == claims.UserID {
		return domain.ErrForbidden
	}
	roles = uniqueSorted(roles)
	if _, err = uuid.Parse(id); err != nil || len(roles) == 0 || len(roles) > 30 {
		return domain.ErrInvalidInput
	}
	return s.authorization.AssignRoles(ctx, claims.UserID, id, roles)
}
func (s *Service) AuthorizationAudit(ctx context.Context, token string, before int64) ([]domain.AuthorizationAudit, error) {
	if _, err := s.RequirePermission(ctx, token, "roles.read"); err != nil {
		return nil, err
	}
	if before < 0 {
		return nil, domain.ErrInvalidInput
	}
	return s.authorization.ListAuthorizationAudit(ctx, before)
}
