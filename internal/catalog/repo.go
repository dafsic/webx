package main

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/dafsic/webx/internal/catalog/model"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/jackc/pgx/v5"
)

const menuTable = "menu_items"

// Repository is the data-access layer for menu items.
type Repository struct {
	db postgres.Database
}

// NewRepository constructs a Repository.
func NewRepository(db postgres.Database) *Repository {
	return &Repository{db: db}
}

// List returns all menu items, optionally restricted to the available ones,
// ordered by id.
func (r *Repository) List(ctx context.Context, availableOnly bool) ([]model.MenuItem, error) {
	q := postgres.Select(postgres.SelectColumns(model.MenuItem{})...).From(menuTable)
	if availableOnly {
		q = q.Where(sq.Eq{"available": true})
	}
	sqlStr, args, err := postgres.ToSQL(q.OrderBy("id"))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: query items: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.MenuItem])
	if err != nil {
		return nil, fmt.Errorf("catalog: scan items: %w", err)
	}
	return items, nil
}

// Get returns the menu item with the given id, or (nil, nil) when not found.
func (r *Repository) Get(ctx context.Context, id int64) (*model.MenuItem, error) {
	q, args, err := postgres.BuildSelect(menuTable, model.MenuItem{}, sq.Eq{"id": id})
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: query item: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.MenuItem])
	if err != nil {
		return nil, fmt.Errorf("catalog: scan item: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

// Create inserts a new menu item and returns the stored row.
func (r *Repository) Create(ctx context.Context, item model.MenuItem) (*model.MenuItem, error) {
	q, args, err := postgres.BuildInsert(menuTable, item)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: insert item: %w", err)
	}
	created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[model.MenuItem])
	if err != nil {
		return nil, fmt.Errorf("catalog: scan inserted item: %w", err)
	}
	return &created, nil
}

// Update overwrites the menu item identified by id with item's non-nil fields,
// bumps updated_at and returns the updated row, or (nil, nil) when not found.
func (r *Repository) Update(ctx context.Context, id int64, item model.MenuItem) (*model.MenuItem, error) {
	cols, vals := postgres.StructColumns(item, postgres.StructOptions{Exclude: []string{"created_at", "updated_at"}})
	q := postgres.Update(menuTable)
	for i, c := range cols {
		q = q.Set(c, vals[i])
	}
	sqlStr, args, err := postgres.ToSQL(q.
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING " + columnList(postgres.SelectColumns(model.MenuItem{}))))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: update item: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.MenuItem])
	if err != nil {
		return nil, fmt.Errorf("catalog: scan updated item: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

// Delete removes the menu item with the given id, reporting whether a row was
// deleted.
func (r *Repository) Delete(ctx context.Context, id int64) (bool, error) {
	q, args, err := postgres.BuildDelete(menuTable, sq.Eq{"id": id})
	if err != nil {
		return false, err
	}
	tag, err := r.db.Pool().Exec(ctx, q, args...)
	if err != nil {
		return false, fmt.Errorf("catalog: delete item: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// columnList joins columns with ", " for a RETURNING clause.
func columnList(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}
