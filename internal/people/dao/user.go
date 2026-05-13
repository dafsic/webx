// Package dao implements persistence for the people domain.
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

// ErrNotFound is returned when a lookup yields zero rows.
var ErrNotFound = errors.New("people: user not found")

// UserDAO is the persistence interface used by the api layer.
type UserDAO interface {
	Create(ctx context.Context, u *model.User) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
}

type userDAO struct {
	db postgres.Database
}

// NewUserDAO wires a UserDAO backed by the given postgres database.
func NewUserDAO(db postgres.Database) UserDAO {
	return &userDAO{db: db}
}

func (d *userDAO) Create(ctx context.Context, u *model.User) (int64, error) {
	q, args, err := postgres.ToSQL(
		postgres.Insert("people").
			Columns("username", "password_hash", "nickname").
			Values(u.Username, u.PasswordHash, u.Nickname).
			Suffix("RETURNING id"),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.Session().GetContext(ctx, &id, q, args...); err != nil {
		return 0, fmt.Errorf("people dao: create: %w", err)
	}
	return id, nil
}

func (d *userDAO) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return d.getOne(ctx, sq.Eq{"id": id})
}

func (d *userDAO) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return d.getOne(ctx, sq.Eq{"username": username})
}

func (d *userDAO) getOne(ctx context.Context, where sq.Sqlizer) (*model.User, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select(
			"id", "username", "password_hash", "nickname", "created_at", "updated_at",
		).From("people").Where(where).Limit(1),
	)
	if err != nil {
		return nil, err
	}
	var u model.User
	if err := d.db.Session().GetContext(ctx, &u, q, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("people dao: get: %w", err)
	}
	return &u, nil
}
