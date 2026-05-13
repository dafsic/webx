// Package model holds the persistent representations of the people domain.
package model

import "time"

// User is the row layout of the `people` table.
//
// `db` tags are consumed by sqlx; `json` tags exist purely for ad-hoc
// debugging — the gRPC layer uses proto_go/people.User instead.
type User struct {
	ID           int64     `db:"id"            json:"id"`
	Username     string    `db:"username"      json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Nickname     string    `db:"nickname"      json:"nickname"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}
