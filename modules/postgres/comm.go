package postgres

import "time"

// Default values and identifiers for the postgres module.
const (
	ModuleName = "postgres"

	DefaultDSN        = "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	DefaultMaxConns   = int32(25)
	DefaultMinConns   = int32(5)
	DefaultConnMaxLifetime = 30 * time.Minute
	DefaultConnMaxIdleTime = 5 * time.Minute
	DefaultPingTimeout     = 5 * time.Second

	DefaultMigrateDir    = "./migrations"
	DefaultMigrateTable  = "migrations"
	DefaultMigrateSchema = "public"

	// MigrateOff disables auto migration on startup (default).
	MigrateOff = "off"
	// MigrateUp applies all pending "up" migrations on startup.
	MigrateUp = "up"
	// MigrateDown reverts the most recently applied migration on startup.
	MigrateDown = "down"

	DefaultMigrate = MigrateOff
)
