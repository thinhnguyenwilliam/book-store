package postgres

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/lib/pq"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
	"gorm.io/gorm"
)

func access(db *gorm.DB, id string) (domain.Access, error) {
	result := domain.Access{AccountID: id, Roles: []string{}, Permissions: []string{}}
	var count int64
	if err := db.Table("auth.accounts").Where("id = ?", id).Count(&count).Error; err != nil {
		return result, err
	}
	if count == 0 {
		return result, domain.ErrNotFound
	}
	if err := db.Table("auth.account_roles").Where("account_id = ?", id).Order("role_code").Pluck("role_code", &result.Roles).Error; err != nil {
		return result, err
	}
	err := db.Raw(`SELECT DISTINCT rp.permission_code FROM auth.role_permissions rp JOIN auth.account_roles ar ON ar.role_code=rp.role_code WHERE ar.account_id=? ORDER BY rp.permission_code`, id).Scan(&result.Permissions).Error
	return result, err
}
func (r *Repository) Access(ctx context.Context, id string) (domain.Access, error) {
	return access(r.db.WithContext(ctx), id)
}

type roleRecord struct {
	Code        string
	Name        string
	Description string
	System      bool
}

func listRoles(db *gorm.DB) ([]domain.Role, error) {
	var records []roleRecord
	if err := db.Table("auth.roles").Order("code").Find(&records).Error; err != nil {
		return nil, err
	}
	type link struct {
		RoleCode       string
		PermissionCode string
	}
	var links []link
	if err := db.Table("auth.role_permissions").Order("permission_code").Find(&links).Error; err != nil {
		return nil, err
	}
	roles := make([]domain.Role, 0, len(records))
	for _, row := range records {
		role := domain.Role{Code: row.Code, Name: row.Name, Description: row.Description, System: row.System, Permissions: []string{}}
		for _, link := range links {
			if link.RoleCode == row.Code {
				role.Permissions = append(role.Permissions, link.PermissionCode)
			}
		}
		roles = append(roles, role)
	}
	return roles, nil
}
func (r *Repository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	return listRoles(r.db.WithContext(ctx))
}
func (r *Repository) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	result := []domain.Permission{}
	err := r.db.WithContext(ctx).Table("auth.permissions").Select(`code,name,description,group_name AS "group"`).Order("group_name,code").Scan(&result).Error
	return result, err
}

// A transaction-scoped lock serializes authorization mutations with last-admin deletion.
func authorizationLock(tx *gorm.DB) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(2026091201)").Error
}
func audit(ctx context.Context, tx *gorm.DB, actor, action, target string, before, after any) error {
	old, err := json.Marshal(before)
	if err != nil {
		return err
	}
	next, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO auth.authorization_audit_logs(actor_id,action,target,before_value,after_value,trace_id) VALUES (?,?,?,?::jsonb,?::jsonb,?)`, actor, action, target, string(old), string(next), apptrace.IDFromContext(ctx)).Error
}
func subset(values, allowed []string) bool {
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
	}
	return true
}
func (r *Repository) SaveRole(ctx context.Context, actor string, role domain.Role, create bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizationLock(tx); err != nil {
			return err
		}
		current, err := access(tx, actor)
		if err != nil {
			return err
		}
		if !slices.Contains(current.Permissions, "roles.manage") {
			return domain.ErrForbidden
		}
		roles, err := listRoles(tx)
		if err != nil {
			return err
		}
		var old *domain.Role
		for i := range roles {
			if roles[i].Code == role.Code {
				old = &roles[i]
				break
			}
		}
		if create && old != nil {
			return domain.ErrInvalidInput
		}
		if !create && old == nil {
			return domain.ErrNotFound
		}
		if old != nil && old.System {
			return domain.ErrProtectedRole
		}
		// Even delegated role editors cannot upgrade a role assigned to themselves.
		if slices.Contains(current.Roles, role.Code) {
			return domain.ErrForbidden
		}
		if !subset(role.Permissions, current.Permissions) || (old != nil && !subset(old.Permissions, current.Permissions)) {
			return domain.ErrForbidden
		}
		var valid []string
		if err := tx.Table("auth.permissions").Pluck("code", &valid).Error; err != nil {
			return err
		}
		if !subset(role.Permissions, valid) {
			return domain.ErrInvalidInput
		}
		role.System = false
		if create {
			if err := tx.Table("auth.roles").Create(&roleRecord{Code: role.Code, Name: role.Name, Description: role.Description}).Error; err != nil {
				return err
			}
		} else if err := tx.Table("auth.roles").Where("code=?", role.Code).Updates(map[string]any{"name": role.Name, "description": role.Description}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM auth.role_permissions WHERE role_code=?", role.Code).Error; err != nil {
			return err
		}
		for _, permission := range role.Permissions {
			if err := tx.Exec("INSERT INTO auth.role_permissions VALUES (?,?) ON CONFLICT DO NOTHING", role.Code, permission).Error; err != nil {
				return err
			}
		}
		return audit(ctx, tx, actor, "role.save", role.Code, old, role)
	})
}
func (r *Repository) DeleteRole(ctx context.Context, actor, code string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizationLock(tx); err != nil {
			return err
		}
		current, err := access(tx, actor)
		if err != nil {
			return err
		}
		if !slices.Contains(current.Permissions, "roles.manage") {
			return domain.ErrForbidden
		}
		roles, err := listRoles(tx)
		if err != nil {
			return err
		}
		var old *domain.Role
		for i := range roles {
			if roles[i].Code == code {
				old = &roles[i]
				break
			}
		}
		if old == nil {
			return domain.ErrNotFound
		}
		if old.System {
			return domain.ErrProtectedRole
		}
		if slices.Contains(current.Roles, code) {
			return domain.ErrForbidden
		}
		if !subset(old.Permissions, current.Permissions) {
			return domain.ErrForbidden
		}
		var used int64
		if err := tx.Table("auth.account_roles").Where("role_code=?", code).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			return domain.ErrRoleInUse
		}
		if err := tx.Exec("DELETE FROM auth.roles WHERE code=?", code).Error; err != nil {
			return err
		}
		return audit(ctx, tx, actor, "role.delete", code, old, nil)
	})
}
func (r *Repository) AssignRoles(ctx context.Context, actor, id string, codes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizationLock(tx); err != nil {
			return err
		}
		current, err := access(tx, actor)
		if err != nil {
			return err
		}
		if actor == id || !slices.Contains(current.Permissions, "users.assign_roles") {
			return domain.ErrForbidden
		}
		old, err := access(tx, id)
		if err != nil {
			return err
		}
		isSuper := slices.Contains(current.Roles, "super_admin")
		if !isSuper && (slices.Contains(old.Roles, "super_admin") || slices.Contains(codes, "super_admin")) {
			return domain.ErrForbidden
		}
		if !subset(old.Permissions, current.Permissions) {
			return domain.ErrForbidden
		}
		roles, err := listRoles(tx)
		if err != nil {
			return err
		}
		for _, code := range codes {
			found := false
			for _, role := range roles {
				if role.Code == code {
					found = true
					if !subset(role.Permissions, current.Permissions) {
						return domain.ErrForbidden
					}
				}
			}
			if !found {
				return domain.ErrInvalidInput
			}
		}
		if slices.Contains(old.Roles, "super_admin") && !slices.Contains(codes, "super_admin") {
			if err := protectLastSuper(tx, id); err != nil {
				return err
			}
		}
		if err := tx.Exec("DELETE FROM auth.account_roles WHERE account_id=?", id).Error; err != nil {
			return err
		}
		for _, code := range codes {
			if err := tx.Exec("INSERT INTO auth.account_roles VALUES (?,?) ON CONFLICT DO NOTHING", id, code).Error; err != nil {
				return err
			}
		}
		// Keep legacy role claims/tools compatible; authorization reads normalized tables.
		if err := tx.Table("auth.accounts").Where("id=?", id).Update("roles", pq.StringArray(codes)).Error; err != nil {
			return err
		}
		return audit(ctx, tx, actor, "account.roles", id, old.Roles, codes)
	})
}
func protectLastSuper(tx *gorm.DB, id string) error {
	var count int64
	if err := tx.Table("auth.account_roles").Where("role_code='super_admin' AND account_id<>?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrProtectedRole
	}
	return nil
}
func (r *Repository) ListAuthorizationAudit(ctx context.Context, before int64) ([]domain.AuthorizationAudit, error) {
	type row struct {
		ID          int64
		ActorID     string
		Action      string
		Target      string
		BeforeValue string
		AfterValue  string
		TraceID     string
		CreatedAt   time.Time
	}
	var records []row
	query := r.db.WithContext(ctx).Table("auth.authorization_audit_logs").Order("id DESC").Limit(50)
	if before > 0 {
		query = query.Where("id < ?", before)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AuthorizationAudit, 0, len(records))
	for _, r := range records {
		result = append(result, domain.AuthorizationAudit{ID: r.ID, ActorID: r.ActorID, Action: r.Action, Target: r.Target, Before: r.BeforeValue, After: r.AfterValue, TraceID: r.TraceID, CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	return result, nil
}
