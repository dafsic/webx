package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" driver for sql-migrate
	migrate "github.com/rubenv/sql-migrate"
)

// driverName is the database/sql driver name used only for sql-migrate.
const driverName = "pgx"

// MigrationResult summarizes a migration run.
type MigrationResult struct {
	Applied int
}

// Migrator wraps github.com/rubenv/sql-migrate with the module's Config.
// A short-lived *sql.DB (via pgx/stdlib) is created only for the duration of
// each migration run; all normal query traffic uses the pgxpool.Pool.
type Migrator struct {
	cfg    *Config
	logger *slog.Logger
}

// NewMigrator constructs a Migrator.
func NewMigrator(cfg *Config, logger *slog.Logger) *Migrator {
	return &Migrator{cfg: cfg, logger: logger}
}

// Up applies all pending migrations.
func (m *Migrator) Up() (*MigrationResult, error) {
	return m.exec(migrate.Up)
}

// Down reverts the most recently applied migration.
func (m *Migrator) Down() (*MigrationResult, error) {
	return m.exec(migrate.Down)
}

// Status returns the records of applied migrations.
func (m *Migrator) Status() ([]*migrate.MigrationRecord, error) {
	if err := m.validateDir(); err != nil {
		return nil, err
	}
	db, err := sql.Open(driverName, m.cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: migrator open: %w", err)
	}
	defer db.Close()
	migrate.SetTable(m.cfg.MigrateTable)
	migrate.SetSchema(m.cfg.MigrateSchema)
	return migrate.GetMigrationRecords(db, driverName)
}

func (m *Migrator) exec(dir migrate.MigrationDirection) (*MigrationResult, error) {
	if err := m.validateDir(); err != nil {
		return nil, err
	}
	db, err := sql.Open(driverName, m.cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: migrator open: %w", err)
	}
	defer db.Close()

	migrate.SetTable(m.cfg.MigrateTable)
	migrate.SetSchema(m.cfg.MigrateSchema)

	source := &migrate.FileMigrationSource{Dir: m.cfg.MigrateDir}

	m.logger.Info("postgres: running migrations",
		"direction", directionName(dir),
		"dir", m.cfg.MigrateDir,
		"table", m.cfg.MigrateTable,
		"schema", m.cfg.MigrateSchema,
	)

	applied, err := migrate.Exec(db, driverName, source, dir)
	if err != nil {
		return nil, fmt.Errorf("postgres: migrate %s: %w", directionName(dir), err)
	}
	m.logger.Info("postgres: migrations finished", "applied", applied)
	return &MigrationResult{Applied: applied}, nil
}

func (m *Migrator) validateDir() error {
	if _, err := os.Stat(m.cfg.MigrateDir); err != nil {
		return fmt.Errorf("postgres: migrate dir %q: %w", m.cfg.MigrateDir, err)
	}
	return nil
}

func directionName(d migrate.MigrationDirection) string {
	if d == migrate.Up {
		return "up"
	}
	return "down"
}
