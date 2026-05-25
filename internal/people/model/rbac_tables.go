package model

import "time"

// User is the row layout of the `people` table.
//
// `db` tags are consumed by sqlx; `json` tags exist purely for ad-hoc
// debugging — the gRPC layer uses proto_go/people.User instead.
type User struct {
	ID        *int64     `db:"id"            json:"id"`
	Address   *string    `db:"address"       json:"address"` // EVM wallet address
	Nickname  *string    `db:"nickname"      json:"nickname"`
	AvatarURL *string    `db:"avatar_url"    json:"avatar_url"`
	Email     *string    `db:"email"         json:"email"`
	LoginIP   *string    `db:"login_ip"      json:"login_ip"`
	CreatedAt *time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"    json:"updated_at"`
}

// Role is a named collection of permissions that can be assigned to users.
type Role struct {
	ID          *int64       `db:"id"          json:"id"`
	Name        *string      `db:"name"        json:"name"`
	Description *string      `db:"description" json:"description"`
	CreatedAt   *time.Time   `db:"created_at"  json:"created_at"`
	Permissions []Permission `db:"-"           json:"permissions"`
}

// Permission represents a single resource+action capability.
type Permission struct {
	ID          *int64     `db:"id"          json:"id"`
	Resource    *string    `db:"resource"    json:"resource"`
	Action      *string    `db:"action"      json:"action"`
	Description *string    `db:"description" json:"description"`
	CreatedAt   *time.Time `db:"created_at"  json:"created_at"`
}
