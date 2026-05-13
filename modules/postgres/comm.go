package postgres

import "time"

// Default values and identifiers for the postgres module.
const (
	ModuleName = "postgres"

	// driverName is the database/sql driver name registered by
	// github.com/jackc/pgx/v5/stdlib. It is intentionally fixed: this module
	// targets PostgreSQL only.
	driverName = "pgx"

	FlagDSN             = "postgres-dsn"
	FlagMaxOpenConns    = "postgres-max-open-conns"
	FlagMaxIdleConns    = "postgres-max-idle-conns"
	FlagConnMaxLifetime = "postgres-conn-max-lifetime"
	FlagConnMaxIdleTime = "postgres-conn-max-idle-time"
	FlagPingTimeout     = "postgres-ping-timeout"

	FlagMigrate       = "postgres-migrate"
	FlagMigrateDir    = "postgres-migrate-dir"
	FlagMigrateTable  = "postgres-migrate-table"
	FlagMigrateSchema = "postgres-migrate-schema"

	EnvDSN             = "POSTGRES_DSN"
	EnvMaxOpenConns    = "POSTGRES_MAX_OPEN_CONNS"
	EnvMaxIdleConns    = "POSTGRES_MAX_IDLE_CONNS"
	EnvConnMaxLifetime = "POSTGRES_CONN_MAX_LIFETIME"
	EnvConnMaxIdleTime = "POSTGRES_CONN_MAX_IDLE_TIME"
	EnvPingTimeout     = "POSTGRES_PING_TIMEOUT"

	EnvMigrate       = "POSTGRES_MIGRATE"
	EnvMigrateDir    = "POSTGRES_MIGRATE_DIR"
	EnvMigrateTable  = "POSTGRES_MIGRATE_TABLE"
	EnvMigrateSchema = "POSTGRES_MIGRATE_SCHEMA"

	DefaultDSN             = "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 5
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

	// OptionsGroup is the fx group tag callers can use to inject extra
	// Options into the postgres configuration.
	OptionsGroup = "postgres_options"
)
