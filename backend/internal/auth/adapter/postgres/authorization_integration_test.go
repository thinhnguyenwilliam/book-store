//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run only against a fresh disposable database. Never use your application DSN.
func TestAuthorizationTransactions(t *testing.T) {
	dsn := os.Getenv("BOOKSTORE_RBAC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated BOOKSTORE_RBAC_TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	var existing bool
	if err := db.Raw("SELECT to_regclass('auth.accounts') IS NOT NULL").Scan(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if existing {
		t.Fatal("refusing nonempty database; use a fresh disposable PostgreSQL instance")
	}
	apply := func(name string) {
		t.Helper()
		data, readErr := os.ReadFile("../../../../migrations/" + name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if execErr := db.Exec(string(data)).Error; execErr != nil {
			t.Fatal(execErr)
		}
	}
	apply("001_init.sql")
	actor, second, target, editor := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{actor, second, target, editor} {
		if err := db.Exec("INSERT INTO auth.accounts(id,email,password_hash,roles) VALUES (?,?,'test',ARRAY['customer'])", id, id+"@rbac.test").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("UPDATE auth.accounts SET roles=ARRAY['admin'] WHERE id=?", actor).Error; err != nil {
		t.Fatal(err)
	}
	apply("020_permissions.sql")
	repository := NewRepository(db)
	initial, err := repository.Access(ctx, actor)
	if err != nil || !slices.Contains(initial.Roles, "super_admin") {
		t.Fatalf("bootstrap failed: %+v %v", initial, err)
	}
	role := domain.Role{Code: "test_catalog", Name: "Test catalog", Permissions: []string{"admin.access", "books.read"}}
	if err := repository.SaveRole(ctx, actor, role, true); err != nil {
		t.Fatal(err)
	}
	if err := repository.AssignRoles(ctx, actor, target, []string{role.Code}); err != nil {
		t.Fatal(err)
	}
	access, err := repository.Access(ctx, target)
	if err != nil || !slices.Contains(access.Permissions, "books.read") || slices.Contains(access.Permissions, "books.update") {
		t.Fatalf("wrong effective permissions: %+v %v", access, err)
	}
	if err := repository.AssignRoles(ctx, target, editor, []string{"admin"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("unprivileged grant: %v", err)
	}
	if err := repository.AssignRoles(ctx, actor, actor, []string{"customer"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self assignment: %v", err)
	}
	if err := repository.SaveRole(ctx, actor, domain.Role{Code: "super_admin", Name: "Changed"}, false); !errors.Is(err, domain.ErrProtectedRole) {
		t.Fatalf("system role edited: %v", err)
	}
	if err := repository.SaveRole(ctx, actor, domain.Role{Code: "unknown_permission", Name: "Bad", Permissions: []string{"not.real"}}, true); err == nil {
		t.Fatal("unknown permission accepted")
	}
	// A delegated editor may only manage permissions they already have, never their own role.
	delegated := domain.Role{Code: "test_editor", Name: "Editor", Permissions: []string{"admin.access", "roles.read", "roles.manage", "users.assign_roles", "books.read"}}
	if err := repository.SaveRole(ctx, actor, delegated, true); err != nil {
		t.Fatal(err)
	}
	if err := repository.AssignRoles(ctx, actor, editor, []string{delegated.Code}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveRole(ctx, editor, delegated, false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self role edit: %v", err)
	}
	if err := repository.AssignRoles(ctx, editor, target, []string{"admin"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("delegated escalation: %v", err)
	}
	role.Permissions = []string{"admin.access"}
	if err := repository.SaveRole(ctx, actor, role, false); err != nil {
		t.Fatal(err)
	}
	access, err = repository.Access(ctx, target)
	if err != nil || slices.Contains(access.Permissions, "books.read") {
		t.Fatal("revocation not immediately effective")
	}
	if err := repository.DeleteRole(ctx, actor, role.Code); !errors.Is(err, domain.ErrRoleInUse) {
		t.Fatalf("assigned role deleted: %v", err)
	}
	if err := repository.AssignRoles(ctx, actor, target, []string{"customer"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteRole(ctx, actor, role.Code); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteRole(ctx, actor, "admin"); !errors.Is(err, domain.ErrProtectedRole) {
		t.Fatalf("system role deleted: %v", err)
	}
	// Protected privileged accounts cannot be deleted through customer administration.
	if err := repository.Delete(domain.ContextWithActor(ctx, editor), actor, time.Now()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("lower privilege deleted admin: %v", err)
	}
	if err := repository.AssignRoles(ctx, actor, second, []string{"super_admin"}); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{actor, second}, {second, actor}} {
		wg.Add(1)
		go func(a, b string) {
			defer wg.Done()
			if repository.AssignRoles(ctx, a, b, []string{"customer"}) == nil {
				successes.Add(1)
			}
		}(pair[0], pair[1])
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent demotions successes=%d", successes.Load())
	}
	var superCount int64
	if err := db.Table("auth.account_roles").Where("role_code='super_admin'").Count(&superCount).Error; err != nil || superCount != 1 {
		t.Fatalf("last admin lost: %d %v", superCount, err)
	}
	apply("020_permissions.sql")
	var after int64
	if err := db.Table("auth.account_roles").Where("role_code='super_admin'").Count(&after).Error; err != nil || after != superCount {
		t.Fatal("migration restored revoked role")
	}
	entries, err := repository.ListAuthorizationAudit(ctx, 0)
	if err != nil || len(entries) < 7 {
		t.Fatalf("audit missing: %d %v", len(entries), err)
	}
}
