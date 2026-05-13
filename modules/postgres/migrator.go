package postgres

import (
	"fmt"
	"log/slog"
	"os"

	migrate "github.com/rubenv/sql-migrate"
)

// MigrationResult summarizes a migration run.
type MigrationResult struct {
	Applied int
}

// Migrator wraps github.com/rubenv/sql-migrate with the module's Config and
// Database. Migration files live in cfg.MigrateDir and follow the standard
// sql-migrate format (`-- +migrate Up` / `-- +migrate Down`).
type Migrator struct {
	db     Database
	logger *slog.Logger
	cfg    *Config
}

// NewMigrator constructs a Migrator. The *slog.Logger must be provided by
// the logs module.
func NewMigrator(db Database, cfg *Config, logger *slog.Logger) *Migrator {
	return &Migrator{db: db, logger: logger, cfg: cfg}
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
	migrate.SetTable(m.cfg.MigrateTable)
	migrate.SetSchema(m.cfg.MigrateSchema)
	return migrate.GetMigrationRecords(m.db.Session().DB, driverName)
}

func (m *Migrator) exec(dir migrate.MigrationDirection) (*MigrationResult, error) {
	if err := m.validateDir(); err != nil {
		return nil, err
	}
	migrate.SetTable(m.cfg.MigrateTable)
	migrate.SetSchema(m.cfg.MigrateSchema)

	source := &migrate.FileMigrationSource{Dir: m.cfg.MigrateDir}

	m.logger.Info("postgres: running migrations",
		"direction", directionName(dir),
		"dir", m.cfg.MigrateDir,
		"table", m.cfg.MigrateTable,
		"schema", m.cfg.MigrateSchema,
	)

	applied, err := migrate.Exec(m.db.Session().DB, driverName, source, dir)
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
