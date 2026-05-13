// Package postgres provides a sqlx + sql-migrate + squirrel based PostgreSQL
// module for the webx framework.
//
// Driver: github.com/jackc/pgx/v5/stdlib (registered as "pgx").
//
// CLI flags registered (all with matching POSTGRES_* environment variables):
//
//	--postgres-dsn                  DSN (default: localhost dev DSN)
//	--postgres-max-open-conns       max open connections (default 25)
//	--postgres-max-idle-conns       max idle connections (default 5)
//	--postgres-conn-max-lifetime    conn max lifetime   (default 30m)
//	--postgres-conn-max-idle-time   conn max idle time  (default 5m)
//	--postgres-ping-timeout         startup ping timeout (default 5s; 0 disables)
//	--postgres-migrate              run "up" migrations on startup (default false)
//	--postgres-migrate-dir          directory with migration files (default ./migrations)
//	--postgres-migrate-table        migration tracking table (default migrations)
//	--postgres-migrate-schema       migration tracking schema (default public)
//
// Provided into the fx graph:
//
//	postgres.Database  — high-level wrapper (Ping/Session/Transact/Close)
//	*sqlx.DB           — underlying handle, for code that needs it directly
//	*postgres.Migrator — sql-migrate driver bound to this database
//	*postgres.Config   — resolved configuration
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
	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" driver
	"github.com/jmoiron/sqlx"
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
			Name:    FlagDSN,
			Value:   DefaultDSN,
			EnvVars: []string{EnvDSN},
			Usage:   "PostgreSQL DSN",
		},
		&cli.IntFlag{
			Name:    FlagMaxOpenConns,
			Value:   DefaultMaxOpenConns,
			EnvVars: []string{EnvMaxOpenConns},
			Usage:   "Maximum number of open connections",
		},
		&cli.IntFlag{
			Name:    FlagMaxIdleConns,
			Value:   DefaultMaxIdleConns,
			EnvVars: []string{EnvMaxIdleConns},
			Usage:   "Maximum number of idle connections",
		},
		&cli.DurationFlag{
			Name:    FlagConnMaxLifetime,
			Value:   DefaultConnMaxLifetime,
			EnvVars: []string{EnvConnMaxLifetime},
			Usage:   "Maximum lifetime of a connection",
		},
		&cli.DurationFlag{
			Name:    FlagConnMaxIdleTime,
			Value:   DefaultConnMaxIdleTime,
			EnvVars: []string{EnvConnMaxIdleTime},
			Usage:   "Maximum idle time of a connection",
		},
		&cli.DurationFlag{
			Name:    FlagPingTimeout,
			Value:   DefaultPingTimeout,
			EnvVars: []string{EnvPingTimeout},
			Usage:   "Startup ping timeout (0 disables)",
		},
		&cli.StringFlag{
			Name:    FlagMigrate,
			Value:   DefaultMigrate,
			EnvVars: []string{EnvMigrate},
			Usage:   "Run migrations on startup: off | up | down",
		},
		&cli.StringFlag{
			Name:    FlagMigrateDir,
			Value:   DefaultMigrateDir,
			EnvVars: []string{EnvMigrateDir},
			Usage:   "Directory containing sql-migrate files",
		},
		&cli.StringFlag{
			Name:    FlagMigrateTable,
			Value:   DefaultMigrateTable,
			EnvVars: []string{EnvMigrateTable},
			Usage:   "Migration tracking table",
		},
		&cli.StringFlag{
			Name:    FlagMigrateSchema,
			Value:   DefaultMigrateSchema,
			EnvVars: []string{EnvMigrateSchema},
			Usage:   "Migration tracking schema",
		},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	cliOpts := []Option{
		WithDSN(ctx.String(FlagDSN)),
		WithMaxOpenConns(ctx.Int(FlagMaxOpenConns)),
		WithMaxIdleConns(ctx.Int(FlagMaxIdleConns)),
		WithConnMaxLifetime(ctx.Duration(FlagConnMaxLifetime)),
		WithConnMaxIdleTime(ctx.Duration(FlagConnMaxIdleTime)),
		WithPingTimeout(ctx.Duration(FlagPingTimeout)),
		WithMigrate(ctx.String(FlagMigrate)),
		WithMigrateDir(ctx.String(FlagMigrateDir)),
		WithMigrateTable(ctx.String(FlagMigrateTable)),
		WithMigrateSchema(ctx.String(FlagMigrateSchema)),
	}

	return fx.Options(
		fx.Supply(fx.Annotate(
			cliOpts,
			fx.ResultTags(`group:"`+OptionsGroup+`,flatten"`),
		)),
		fx.Provide(fx.Annotate(
			NewConfig,
			fx.ParamTags(`group:"`+OptionsGroup+`"`),
		)),
		fx.Provide(
			NewDatabase,
			NewMigrator,
			func(db Database) *sqlx.DB { return db.Session() },
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
