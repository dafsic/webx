package main

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/dafsic/webx/internal/orders/model"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	ordersTable     = "orders"
	orderItemsTable = "order_items"
)

// Repository is the data-access layer for orders.
type Repository struct {
	db postgres.Database
}

// NewRepository constructs a Repository.
func NewRepository(db postgres.Database) *Repository {
	return &Repository{db: db}
}

// Create inserts an order together with its items in a single transaction and
// returns the stored order and items.
func (r *Repository) Create(ctx context.Context, order model.Order, items []model.OrderItem) (*model.Order, []model.OrderItem, error) {
	var (
		stored      model.Order
		storedItems []model.OrderItem
	)
	err := r.db.Transact(ctx, func(tx pgx.Tx) error {
		q, args, err := postgres.BuildInsert(ordersTable, order)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("orders: insert order: %w", err)
		}
		stored, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[model.Order])
		if err != nil {
			return fmt.Errorf("orders: scan inserted order: %w", err)
		}

		for i := range items {
			items[i].OrderID = stored.ID
			iq, iargs, err := postgres.BuildInsert(orderItemsTable, items[i])
			if err != nil {
				return err
			}
			irows, err := tx.Query(ctx, iq, iargs...)
			if err != nil {
				return fmt.Errorf("orders: insert order item: %w", err)
			}
			stItem, err := pgx.CollectExactlyOneRow(irows, pgx.RowToStructByName[model.OrderItem])
			if err != nil {
				return fmt.Errorf("orders: scan inserted order item: %w", err)
			}
			storedItems = append(storedItems, stItem)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &stored, storedItems, nil
}

// Get returns the order with the given id and its items, or (nil, nil, nil)
// when not found.
func (r *Repository) Get(ctx context.Context, id int64) (*model.Order, []model.OrderItem, error) {
	q, args, err := postgres.BuildSelect(ordersTable, model.Order{}, sq.Eq{"id": id})
	if err != nil {
		return nil, nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: query order: %w", err)
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])
	if err != nil {
		return nil, nil, fmt.Errorf("orders: scan order: %w", err)
	}
	if len(orders) == 0 {
		return nil, nil, nil
	}
	items, err := r.itemsFor(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &orders[0], items, nil
}

// List returns orders ordered by newest first, optionally filtered by status,
// each with its items attached.
func (r *Repository) List(ctx context.Context, status string) ([]model.Order, map[int64][]model.OrderItem, error) {
	b := postgres.Select(postgres.SelectColumns(model.Order{})...).From(ordersTable)
	if status != "" {
		b = b.Where(sq.Eq{"status": status})
	}
	sqlStr, args, err := postgres.ToSQL(b.OrderBy("id DESC"))
	if err != nil {
		return nil, nil, err
	}
	rows, err := r.db.Pool().Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: query orders: %w", err)
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])
	if err != nil {
		return nil, nil, fmt.Errorf("orders: scan orders: %w", err)
	}
	if len(orders) == 0 {
		return orders, map[int64][]model.OrderItem{}, nil
	}

	ids := make([]int64, 0, len(orders))
	for i := range orders {
		if orders[i].ID != nil {
			ids = append(ids, *orders[i].ID)
		}
	}
	itemRows, err := r.db.Pool().Query(ctx,
		"SELECT "+columnList(postgres.SelectColumns(model.OrderItem{}))+
			" FROM "+orderItemsTable+" WHERE order_id = ANY($1) ORDER BY id", ids)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: query order items: %w", err)
	}
	allItems, err := pgx.CollectRows(itemRows, pgx.RowToStructByName[model.OrderItem])
	if err != nil {
		return nil, nil, fmt.Errorf("orders: scan order items: %w", err)
	}
	byOrder := make(map[int64][]model.OrderItem, len(orders))
	for _, it := range allItems {
		if it.OrderID != nil {
			byOrder[*it.OrderID] = append(byOrder[*it.OrderID], it)
		}
	}
	return orders, byOrder, nil
}

// UpdateStatus sets the status of an order, bumps updated_at and returns the
// updated order with its items, or (nil, nil, nil) when not found.
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string) (*model.Order, []model.OrderItem, error) {
	sqlStr, args, err := postgres.ToSQL(postgres.Update(ordersTable).
		Set("status", status).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING " + columnList(postgres.SelectColumns(model.Order{}))))
	if err != nil {
		return nil, nil, err
	}
	rows, err := r.db.Pool().Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("orders: update order status: %w", err)
	}
	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])
	if err != nil {
		return nil, nil, fmt.Errorf("orders: scan updated order: %w", err)
	}
	if len(orders) == 0 {
		return nil, nil, nil
	}
	items, err := r.itemsFor(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &orders[0], items, nil
}

// itemsFor returns the items of a single order ordered by id.
func (r *Repository) itemsFor(ctx context.Context, orderID int64) ([]model.OrderItem, error) {
	q, args, err := postgres.ToSQL(
		postgres.Select(postgres.SelectColumns(model.OrderItem{})...).
			From(orderItemsTable).
			Where(sq.Eq{"order_id": orderID}).
			OrderBy("id"))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Pool().Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("orders: query order items: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.OrderItem])
	if err != nil {
		return nil, fmt.Errorf("orders: scan order items: %w", err)
	}
	return items, nil
}

// columnList joins columns with ", " for a RETURNING / SELECT clause.
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
