// Package postgres provides a native pgx v5 + sql-migrate + squirrel based
// PostgreSQL module for the webx framework.
//
// Driver: github.com/jackc/pgx/v5/pgxpool (native pgx — no database/sql).
// Migrations only: pgx/stdlib *sql.DB is opened temporarily for sql-migrate.
//
// CLI flags registered (all with matching POSTGRES_* environment variables):
//
//	--postgres-dsn                  DSN (default: localhost dev DSN)
//	--postgres-max-conns            max pool connections (default 25)
//	--postgres-min-conns            min idle connections (default 5)
//	--postgres-max-conn-lifetime    max connection lifetime (default 30m)
//	--postgres-max-conn-idle-time   max connection idle time (default 5m)
//	--postgres-ping-timeout         startup ping timeout (default 5s; 0 disables)
//	--postgres-migrate              run "up" migrations on startup (default false)
//	--postgres-migrate-dir          directory with migration files (default ./migrations)
//	--postgres-migrate-table        migration tracking table (default migrations)
//	--postgres-migrate-schema       migration tracking schema (default public)
//
// Provided into the fx graph:
//
//	postgres.Database    — high-level wrapper (Ping/Pool/Transact/Close)
//	*pgxpool.Pool        — underlying pool, for code that needs it directly
//	*postgres.Migrator   — sql-migrate driver bound to this database config
//	*postgres.Config     — resolved configuration
//
// Lifecycle:
//
//	OnStart: Ping (subject to PingTimeout), then optional auto-migrate.
//	OnStop:  Close the connection pool.
//
// Extending: callers can contribute extra Options via the
// group:"postgres_options" fx group.
package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dafsic/webx/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// Module is the postgres module.
type Module struct{}

// New returns a new postgres Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{
			Name:    "postgres-dsn",
			Value:   DefaultDSN,
			EnvVars: []string{"POSTGRES_DSN"},
			Usage:   "PostgreSQL DSN",
		},
		&cli.IntFlag{
			Name:    "postgres-max-conns",
			Value:   int(DefaultMaxConns),
			EnvVars: []string{"POSTGRES_MAX_CONNS"},
			Usage:   "Maximum number of connections in the pool",
		},
		&cli.IntFlag{
			Name:    "postgres-min-conns",
			Value:   int(DefaultMinConns),
			EnvVars: []string{"POSTGRES_MIN_CONNS"},
			Usage:   "Minimum number of idle connections",
		},
		&cli.DurationFlag{
			Name:    "postgres-max-conn-lifetime",
			Value:   DefaultConnMaxLifetime,
			EnvVars: []string{"POSTGRES_MAX_CONN_LIFETIME"},
			Usage:   "Maximum lifetime of a connection",
		},
		&cli.DurationFlag{
			Name:    "postgres-max-conn-idle-time",
			Value:   DefaultConnMaxIdleTime,
			EnvVars: []string{"POSTGRES_MAX_CONN_IDLE_TIME"},
			Usage:   "Maximum idle time before a connection is evicted",
		},
		&cli.DurationFlag{
			Name:    "postgres-ping-timeout",
			Value:   DefaultPingTimeout,
			EnvVars: []string{"POSTGRES_PING_TIMEOUT"},
			Usage:   "Startup ping timeout (0 disables)",
		},
		&cli.StringFlag{
			Name:    "postgres-migrate",
			Value:   DefaultMigrate,
			EnvVars: []string{"POSTGRES_MIGRATE"},
			Usage:   "Run migrations on startup: off | up | down",
		},
		&cli.StringFlag{
			Name:    "postgres-migrate-dir",
			Value:   DefaultMigrateDir,
			EnvVars: []string{"POSTGRES_MIGRATE_DIR"},
			Usage:   "Directory containing sql-migrate files",
		},
		&cli.StringFlag{
			Name:    "postgres-migrate-table",
			Value:   DefaultMigrateTable,
			EnvVars: []string{"POSTGRES_MIGRATE_TABLE"},
			Usage:   "Migration tracking table",
		},
		&cli.StringFlag{
			Name:    "postgres-migrate-schema",
			Value:   DefaultMigrateSchema,
			EnvVars: []string{"POSTGRES_MIGRATE_SCHEMA"},
			Usage:   "Migration tracking schema",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cliOpts := []Option{
		WithDSN(ctx.String("postgres-dsn")),
		WithMaxConns(int32(ctx.Int("postgres-max-conns"))),
		WithMinConns(int32(ctx.Int("postgres-min-conns"))),
		WithMaxConnLifetime(ctx.Duration("postgres-max-conn-lifetime")),
		WithMaxConnIdleTime(ctx.Duration("postgres-max-conn-idle-time")),
		WithPingTimeout(ctx.Duration("postgres-ping-timeout")),
		WithMigrate(ctx.String("postgres-migrate")),
		WithMigrateDir(ctx.String("postgres-migrate-dir")),
		WithMigrateTable(ctx.String("postgres-migrate-table")),
		WithMigrateSchema(ctx.String("postgres-migrate-schema")),
	}

	return fx.Options(
		fx.Supply(fx.Annotate(
			cliOpts,
			fx.ResultTags(`group:"postgres_options,flatten"`),
		)),
		fx.Provide(fx.Annotate(
			NewConfig,
			fx.ParamTags(`group:"postgres_options"`),
		)),
		fx.Provide(
			NewDatabase,
			NewMigrator,
			func(db Database) *pgxpool.Pool { return db.Pool() },
		),
		fx.Invoke(registerLifecycle),
	)
}

// registerLifecycle wires Ping, optional auto-migration, and Close into the
// fx lifecycle. The *slog.Logger is supplied by the logs module.
func registerLifecycle(lc fx.Lifecycle, db Database, mig *Migrator, cfg *Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			if cfg.PingTimeout > 0 {
				pingCtx, cancel := context.WithTimeout(startCtx, cfg.PingTimeout)
				defer cancel()
				if err := db.Ping(pingCtx); err != nil {
					_ = db.Close()
					return err
				}
			}
			switch cfg.Migrate {
			case "", MigrateOff:
			case MigrateUp:
				logger.Info("postgres: auto-migrate up")
				if _, err := mig.Up(); err != nil {
					_ = db.Close()
					return fmt.Errorf("postgres: auto-migrate up: %w", err)
				}
			case MigrateDown:
				logger.Info("postgres: auto-migrate down")
				if _, err := mig.Down(); err != nil {
					_ = db.Close()
					return fmt.Errorf("postgres: auto-migrate down: %w", err)
				}
			default:
				_ = db.Close()
				return fmt.Errorf("postgres: invalid migrate mode %q", cfg.Migrate)
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return db.Close()
		},
	})
}
