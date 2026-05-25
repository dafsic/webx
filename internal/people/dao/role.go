package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/dafsic/webx/internal/people/model"
	"github.com/dafsic/webx/modules/postgres"
)

// ── Role CRUD ──────────────────────────────────────────────────────────────

func (d *DAO) CreateRole(ctx context.Context, r *model.Role) (int64, error) {
	q, args, err := postgres.ToSQL(
		postgres.Insert("roles").
			Columns("name", "description").
			Values(r.Name, r.Description).
			Suffix("ON CONFLICT (name) DO NOTHING RETURNING id"),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.Session().GetContext(ctx, &id, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Already exists – look it up.
			return d.getRoleIDByName(ctx, r.Name)
		}
		return 0, fmt.Errorf("people dao: create role: %w", err)
	}
	return id, nil
}

func (d *roleDAO) GetRoleByName(ctx context.Context, name string) (*model.Role, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select("id", "name", "description", "created_at").
			From("roles").Where(sq.Eq{"name": name}).Limit(1),
	)
	if err != nil {
		return nil, err
	}
	var r model.Role
	if err := d.db.Session().GetContext(ctx, &r, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("people dao: get role: %w", err)
	}
	return &r, nil
}

func (d *roleDAO) getRoleIDByName(ctx context.Context, name string) (int64, error) {
	r, err := d.GetRoleByName(ctx, name)
	if err != nil {
		return 0, err
	}
	return r.ID, nil
}

// GetRolesByUserID returns all roles the user holds, each with its
// permissions eagerly loaded.
func (d *roleDAO) GetRolesByUserID(ctx context.Context, userID int64) ([]model.Role, error) {
	// Step 1: fetch roles for the user.
	rq, rargs, err := postgres.ToSQL(
		postgres.Select("r.id", "r.name", "r.description", "r.created_at").
			From("roles r").
			Join("user_roles ur ON ur.role_id = r.id").
			Where(sq.Eq{"ur.user_id": userID}),
	)
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	if err := d.db.Session().SelectContext(ctx, &roles, rq, rargs...); err != nil {
		return nil, fmt.Errorf("people dao: get roles: %w", err)
	}
	if len(roles) == 0 {
		return roles, nil
	}

	// Step 2: collect role IDs and fetch permissions in one query.
	roleIDs := make([]int64, len(roles))
	for i, r := range roles {
		roleIDs[i] = r.ID
	}

	type permRow struct {
		RoleID int64 `db:"role_id"`
		model.Permission
	}
	pq, pargs, err := postgres.ToSQL(
		postgres.Select(
			"rp.role_id",
			"p.id", "p.resource", "p.action", "p.description", "p.created_at",
		).
			From("permissions p").
			Join("role_permissions rp ON rp.permission_id = p.id").
			Where(sq.Eq{"rp.role_id": roleIDs}),
	)
	if err != nil {
		return nil, err
	}
	var permRows []permRow
	if err := d.db.Session().SelectContext(ctx, &permRows, pq, pargs...); err != nil {
		return nil, fmt.Errorf("people dao: get permissions: %w", err)
	}

	// Step 3: map permissions back to roles.
	permsByRole := make(map[int64][]model.Permission, len(roles))
	for _, pr := range permRows {
		permsByRole[pr.RoleID] = append(permsByRole[pr.RoleID], pr.Permission)
	}
	for i := range roles {
		roles[i].Permissions = permsByRole[roles[i].ID]
	}
	return roles, nil
}

func (d *roleDAO) AssignRole(ctx context.Context, userID, roleID int64) error {
	q, args, err := postgres.ToSQL(
		postgres.Insert("user_roles").
			Columns("user_id", "role_id").
			Values(userID, roleID).
			Suffix("ON CONFLICT DO NOTHING"),
	)
	if err != nil {
		return err
	}
	if _, err := d.db.Session().ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("people dao: assign role: %w", err)
	}
	return nil
}

func (d *roleDAO) RevokeRole(ctx context.Context, userID, roleID int64) error {
	q, args, err := postgres.ToSQL(
		postgres.Delete("user_roles").
			Where(sq.Eq{"user_id": userID, "role_id": roleID}),
	)
	if err != nil {
		return err
	}
	if _, err := d.db.Session().ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("people dao: revoke role: %w", err)
	}
	return nil
}

// ── Permission CRUD ────────────────────────────────────────────────────────

func (d *roleDAO) CreatePermission(ctx context.Context, p *model.Permission) (int64, error) {
	q, args, err := postgres.ToSQL(
		postgres.Insert("permissions").
			Columns("resource", "action", "description").
			Values(p.Resource, p.Action, p.Description).
			Suffix("ON CONFLICT (resource, action) DO NOTHING RETURNING id"),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.Session().GetContext(ctx, &id, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Already exists – fetch it.
			return d.getPermissionID(ctx, p.Resource, p.Action)
		}
		return 0, fmt.Errorf("people dao: create permission: %w", err)
	}
	return id, nil
}

func (d *roleDAO) getPermissionID(ctx context.Context, resource, action string) (int64, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select("id").From("permissions").
			Where(sq.Eq{"resource": resource, "action": action}).Limit(1),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.Session().GetContext(ctx, &id, q, args...); err != nil {
		return 0, fmt.Errorf("people dao: get permission id: %w", err)
	}
	return id, nil
}

func (d *roleDAO) AttachPermission(ctx context.Context, roleID, permissionID int64) error {
	q, args, err := postgres.ToSQL(
		postgres.Insert("role_permissions").
			Columns("role_id", "permission_id").
			Values(roleID, permissionID).
			Suffix("ON CONFLICT DO NOTHING"),
	)
	if err != nil {
		return err
	}
	if _, err := d.db.Session().ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("people dao: attach permission: %w", err)
	}
	return nil
}

func (d *roleDAO) HasPermission(ctx context.Context, userID int64, resource, action string) (bool, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select("1").
			From("user_roles ur").
			Join("role_permissions rp ON rp.role_id = ur.role_id").
			Join("permissions p ON p.id = rp.permission_id").
			Where(sq.Eq{
				"ur.user_id": userID,
				"p.resource": resource,
				"p.action":   action,
			}).Limit(1),
	)
	if err != nil {
		return false, err
	}
	var dummy int
	err = d.db.Session().GetContext(ctx, &dummy, q, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("people dao: has permission: %w", err)
	}
	return true, nil
}
