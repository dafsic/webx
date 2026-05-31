package main

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/dafsic/webx/internal/people/model"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/jackc/pgx/v5"
)

// Repository is the data-access layer for accounts and RBAC.
type Repository struct {
	db postgres.Database
}

// NewRepository constructs a Repository.
func NewRepository(db postgres.Database) *Repository {
	return &Repository{db: db}
}

// FindByAddress returns the account for the given (already normalized) address,
// or (nil, nil) when no such account exists.
func (r *Repository) FindByAddress(ctx context.Context, address string) (*model.User, error) {
	q, args, err := postgres.BuildSelect("people", model.User{}, sq.Eq{"address": address})
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("people: query user: %w", err)
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, fmt.Errorf("people: scan user: %w", err)
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

// Create inserts a new account for address and returns the stored row.
func (r *Repository) Create(ctx context.Context, address string) (*model.User, error) {
	q, args, err := postgres.BuildInsert("people", model.User{Address: &address})
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("people: insert user: %w", err)
	}
	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, fmt.Errorf("people: scan inserted user: %w", err)
	}
	return &user, nil
}

// HasPermission reports whether user holds the (resource, action) permission
// through any of its roles.
func (r *Repository) HasPermission(ctx context.Context, userID int64, resource, action string) (bool, error) {
	const query = `
SELECT EXISTS (
    SELECT 1
    FROM user_roles ur
    JOIN role_permissions rp ON rp.role_id = ur.role_id
    JOIN permissions p       ON p.id = rp.permission_id
    WHERE ur.user_id = $1 AND p.resource = $2 AND p.action = $3
)`
	var ok bool
	if err := r.db.Pool().QueryRow(ctx, query, userID, resource, action).Scan(&ok); err != nil {
		return false, fmt.Errorf("people: check permission: %w", err)
	}
	return ok, nil
}

// RolesWithPermissions returns the roles assigned to user, each populated with
// its permissions. Intended for building the login response.
func (r *Repository) RolesWithPermissions(ctx context.Context, userID int64) ([]model.Role, error) {
	const rolesQuery = `
SELECT r.id, r.name, r.description, r.created_at
FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
ORDER BY r.id`
	rows, err := r.db.Pool().Query(ctx, rolesQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("people: query roles: %w", err)
	}
	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("people: scan role: %w", err)
		}
		roles = append(roles, role)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("people: iterate roles: %w", err)
	}

	for i := range roles {
		perms, err := r.permissionsForRole(ctx, *roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = perms
	}
	return roles, nil
}

// permissionsForRole returns all permissions granted to a role.
func (r *Repository) permissionsForRole(ctx context.Context, roleID int64) ([]model.Permission, error) {
	const query = `
SELECT p.id, p.resource, p.action, p.description, p.created_at
FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = $1
ORDER BY p.id`
	rows, err := r.db.Pool().Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("people: query permissions: %w", err)
	}
	defer rows.Close()
	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("people: scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("people: iterate permissions: %w", err)
	}
	return perms, nil
}
