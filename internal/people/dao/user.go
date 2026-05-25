// Package dao implements persistence for the people domain.
package dao

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/dafsic/webx/internal/people/model"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a lookup yields zero rows.
var ErrNotFound = errors.New("people: record not found")

var userColumns = []string{
	"id", "address", "nickname", "avatar_url", "email",
	"created_at", "updated_at",
}

func (d *DAO) Create(ctx context.Context, u *model.User) (int64, error) {
	q, args, err := postgres.ToSQL(
		postgres.Insert("people").
			Columns("address", "nickname", "avatar_url", "email").
			Values(u.Address, u.Nickname, u.AvatarURL, u.Email).
			Suffix("RETURNING id"),
	)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := d.db.Pool().QueryRow(ctx, q, args...).Scan(&id); err != nil {
		return 0, fmt.Errorf("people dao: create: %w", err)
	}
	return id, nil
}

func (d *DAO) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return d.getOne(ctx, sq.Eq{"id": id})
}

func (d *DAO) GetByAddress(ctx context.Context, address string) (*model.User, error) {
	return d.getOne(ctx, sq.Eq{"address": address})
}

func (d *DAO) Update(ctx context.Context, u *model.User) error {
	q, args, err := postgres.ToSQL(
		postgres.Update("people").
			Set("nickname", u.Nickname).
			Set("avatar_url", u.AvatarURL).
			Set("email", u.Email).
			Set("updated_at", sq.Expr("NOW()")).
			Where(sq.Eq{"id": u.ID}),
	)
	if err != nil {
		return err
	}
	if _, err := d.db.Pool().Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("people dao: update: %w", err)
	}
	return nil
}

// getOrCreateSQL uses an upsert so the operation is atomic even under
// concurrent login attempts for the same address.
const getOrCreateSQL = `
INSERT INTO people (address)
VALUES ($1)
ON CONFLICT (address) DO UPDATE SET updated_at = updated_at
RETURNING id, address, nickname, avatar_url, email, created_at, updated_at`

func (d *DAO) GetOrCreateByAddress(ctx context.Context, address string) (*model.User, bool, error) {
	// Run a SELECT first so we can tell whether the row is new.
	existing, err := d.GetByAddress(ctx, address)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}
	// Not found → upsert. A concurrent INSERT may win the race; the ON CONFLICT
	// clause guarantees we still get the row back.
	rows, err := d.db.Pool().Query(ctx, getOrCreateSQL, address)
	if err != nil {
		return nil, false, fmt.Errorf("people dao: get-or-create: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, false, fmt.Errorf("people dao: get-or-create: %w", err)
	}
	return &u, true, nil
}

func (d *DAO) getOne(ctx context.Context, where sq.Sqlizer) (*model.User, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select(userColumns...).From("people").Where(where).Limit(1),
	)
	if err != nil {
		return nil, err
	}
	rows, err := d.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("people dao: get: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("people dao: get: %w", err)
	}
	return &u, nil
}
