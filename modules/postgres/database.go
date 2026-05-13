package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// Database is the high-level abstraction exposed by the module.
type Database interface {
	// Ping checks connectivity.
	Ping(ctx context.Context) error
	// Session returns the underlying *sqlx.DB.
	Session() *sqlx.DB
	// Transact runs txFunc inside a read-write transaction. The transaction
	// is rolled back if txFunc returns an error or panics, and committed
	// otherwise.
	Transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error
	// TransactReadOnly runs txFunc inside a read-only transaction.
	TransactReadOnly(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error
	// Close releases the underlying connection pool.
	Close() error
}

type database struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewDatabase opens a *sqlx.DB using the pgx stdlib driver and returns a
// Database wrapper. The pool tuning knobs from cfg are applied immediately.
//
// The *slog.Logger must be provided by the logs module (or any other
// fx.Provide of *slog.Logger); this module does not fall back to
// slog.Default.
func NewDatabase(cfg *Config, logger *slog.Logger) (Database, error) {
	db, err := sqlx.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	return &database{db: db, logger: logger}, nil
}

func (d *database) Ping(ctx context.Context) error {
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}

func (d *database) Session() *sqlx.DB { return d.db }

func (d *database) Close() error { return d.db.Close() }

func (d *database) Transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error {
	return d.transact(ctx, txFunc, nil)
}

func (d *database) TransactReadOnly(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error {
	return d.transact(ctx, txFunc, &sql.TxOptions{ReadOnly: true})
}

func (d *database) transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error, opts *sql.TxOptions) (err error) {
	tx, err := d.db.BeginTxx(ctx, opts)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() {
		switch p := recover(); {
		case p != nil:
			if rbErr := tx.Rollback(); rbErr != nil {
				d.logger.Error("postgres: rollback after panic failed", "err", rbErr)
			}
			panic(p)
		case err != nil:
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v: %w", rbErr, err)
			}
		default:
			if cErr := tx.Commit(); cErr != nil {
				err = fmt.Errorf("postgres: commit: %w", cErr)
			}
		}
	}()
	err = txFunc(tx)
	return
}
