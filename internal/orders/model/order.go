package model

import "time"

// Order is the row layout of the `orders` table.
type Order struct {
	ID         *int64     `db:"id"          json:"id"`
	TableNo    *string    `db:"table_no"    json:"table_no"`
	Status     *string    `db:"status"      json:"status"`
	TotalCents *int64     `db:"total_cents" json:"total_cents"`
	Note       *string    `db:"note"        json:"note"`
	CreatedAt  *time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"  json:"updated_at"`
}

// OrderItem is the row layout of the `order_items` table.
type OrderItem struct {
	ID             *int64  `db:"id"               json:"id"`
	OrderID        *int64  `db:"order_id"         json:"order_id"`
	MenuItemID     *int64  `db:"menu_item_id"     json:"menu_item_id"`
	Name           *string `db:"name"             json:"name"`
	UnitPriceCents *int64  `db:"unit_price_cents" json:"unit_price_cents"`
	Quantity       *int32  `db:"quantity"         json:"quantity"`
	SubtotalCents  *int64  `db:"subtotal_cents"   json:"subtotal_cents"`
}
