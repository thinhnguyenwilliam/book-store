package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type authorizationRepositoryStub struct {
	access      domain.Access
	saved       domain.Role
	deleted     string
	assigned    []string
	assignTo    string
	saveErr     error
	deleteErr   error
	assignErr   error
	roles       []domain.Role
	permissions []domain.Permission
}

func (r *authorizationRepositoryStub) Access(context.Context, string) (domain.Access, error) {
	return r.access, nil
}
func (r *authorizationRepositoryStub) ListRoles(context.Context) ([]domain.Role, error) {
	return r.roles, nil
}
func (r *authorizationRepositoryStub) ListPermissions(context.Context) ([]domain.Permission, error) {
	return r.permissions, nil
}
func (r *authorizationRepositoryStub) SaveRole(_ context.Context, _ string, role domain.Role, _ bool) error {
	r.saved = role
	return r.saveErr
}
func (r *authorizationRepositoryStub) DeleteRole(_ context.Context, _, code string) error {
	r.deleted = code
	return r.deleteErr
}
func (r *authorizationRepositoryStub) AssignRoles(_ context.Context, _, id string, roles []string) error {
	r.assignTo, r.assigned = id, roles
	return r.assignErr
}
func (r *authorizationRepositoryStub) ListAuthorizationAudit(context.Context, int64) ([]domain.AuthorizationAudit, error) {
	return nil, nil
}

func newAuthorizationService(access domain.Access) (*Service, *authorizationRepositoryStub) {
	account := &domain.Account{ID: "user-id", Email: "admin@example.com", Roles: access.Roles}
	authz := &authorizationRepositoryStub{access: access}
	service := newTestService(&accountRepositoryStub{account: account})
	service.SetAuthorizationRepository(authz)
	return service, authz
}

func TestSaveRoleNormalizesPermissions(t *testing.T) {
	service, authz := newAuthorizationService(domain.Access{
		AccountID: "user-id", Roles: []string{"super_admin"}, Permissions: []string{"roles.manage", "admin.access", "books.read"},
	})
	err := service.SaveRole(context.Background(), "token", domain.Role{
		Code: " warehouse_staff ", Name: " Kho ", Permissions: []string{"books.read", "admin.access", "books.read", ""},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if authz.saved.Code != "warehouse_staff" || authz.saved.Name != "Kho" {
		t.Fatalf("role not trimmed: %+v", authz.saved)
	}
	if !slices.Equal(authz.saved.Permissions, []string{"admin.access", "books.read"}) {
		t.Fatalf("permissions = %v", authz.saved.Permissions)
	}
}

func TestSaveRoleRejectsEmptyPermissions(t *testing.T) {
	service, _ := newAuthorizationService(domain.Access{Permissions: []string{"roles.manage"}})
	if err := service.SaveRole(context.Background(), "token", domain.Role{Code: "empty_role", Name: "Empty"}, true); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty permissions: %v", err)
	}
}

func TestAssignRolesRejectsSelfAndDeduplicates(t *testing.T) {
	service, authz := newAuthorizationService(domain.Access{Permissions: []string{"users.assign_roles"}})
	if err := service.AssignRoles(context.Background(), "token", "user-id", []string{"customer"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self assign: %v", err)
	}
	target := uuid.NewString()
	if err := service.AssignRoles(context.Background(), "token", target, []string{"customer", " customer ", "customer"}); err != nil {
		t.Fatal(err)
	}
	if authz.assignTo != target || !slices.Equal(authz.assigned, []string{"customer"}) {
		t.Fatalf("assigned = %s %v", authz.assignTo, authz.assigned)
	}
}

func TestDeleteRoleRequiresManagePermission(t *testing.T) {
	service, _ := newAuthorizationService(domain.Access{Permissions: []string{"roles.read"}})
	if err := service.DeleteRole(context.Background(), "token", "test_catalog"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("missing manage: %v", err)
	}
	service, authz := newAuthorizationService(domain.Access{Permissions: []string{"roles.manage"}})
	if err := service.DeleteRole(context.Background(), "token", "test_catalog"); err != nil {
		t.Fatal(err)
	}
	if authz.deleted != "test_catalog" {
		t.Fatalf("deleted = %q", authz.deleted)
	}
}

func TestUniqueSorted(t *testing.T) {
	got := uniqueSorted([]string{" books.read ", "admin.access", "books.read", "", "admin.access"})
	if !slices.Equal(got, []string{"admin.access", "books.read"}) {
		t.Fatalf("got %v", got)
	}
}
