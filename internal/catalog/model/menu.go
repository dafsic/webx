package model

import "time"

// MenuItem is the row layout of the `menu_items` table.
//
// `db` tags are consumed by the postgres CRUD helpers and pgx's
// RowToStructByName; the gRPC layer uses proto_go/catalog.MenuItem instead.
type MenuItem struct {
	ID          *int64     `db:"id"          json:"id"`
	Name        *string    `db:"name"        json:"name"`
	Description *string    `db:"description" json:"description"`
	PriceCents  *int64     `db:"price_cents" json:"price_cents"`
	Available   *bool      `db:"available"   json:"available"`
	CreatedAt   *time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"  json:"updated_at"`
}
