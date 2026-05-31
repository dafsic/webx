package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Database is the high-level abstraction exposed by the module.
type Database interface {
	// Ping checks connectivity.
	Ping(ctx context.Context) error
	// Pool returns the underlying *pgxpool.Pool.
	Pool() *pgxpool.Pool
	// Transact runs txFunc inside a read-write transaction. The transaction
	// is rolled back if txFunc returns an error or panics, and committed
	// otherwise.
	Transact(ctx context.Context, txFunc func(tx pgx.Tx) error) error
	// TransactReadOnly runs txFunc inside a read-only transaction.
	TransactReadOnly(ctx context.Context, txFunc func(tx pgx.Tx) error) error
	// Close releases the underlying connection pool.
	Close() error
}

type database struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewDatabase creates a pgxpool.Pool from cfg and returns a Database wrapper.
//
// The *slog.Logger must be provided by the logs module (or any other
// fx.Provide of *slog.Logger); this module does not fall back to
// slog.Default.
func NewDatabase(cfg *Config, logger *slog.Logger) (Database, error) {
	pCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dsn: %w", err)
	}
	pCfg.MaxConns = cfg.MaxConns
	pCfg.MinConns = cfg.MinConns
	pCfg.MaxConnLifetime = cfg.MaxConnLifetime
	pCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(context.Background(), pCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	return &database{pool: pool, logger: logger}, nil
}

func (d *database) Ping(ctx context.Context) error {
	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}

func (d *database) Pool() *pgxpool.Pool { return d.pool }

func (d *database) Close() error {
	d.pool.Close()
	return nil
}

func (d *database) Transact(ctx context.Context, txFunc func(tx pgx.Tx) error) error {
	return d.transact(ctx, txFunc, pgx.TxOptions{})
}

func (d *database) TransactReadOnly(ctx context.Context, txFunc func(tx pgx.Tx) error) error {
	return d.transact(ctx, txFunc, pgx.TxOptions{AccessMode: pgx.ReadOnly})
}

func (d *database) transact(ctx context.Context, txFunc func(tx pgx.Tx) error, opts pgx.TxOptions) (err error) {
	tx, err := d.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() {
		switch p := recover(); {
		case p != nil:
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				d.logger.Error("postgres: rollback after panic failed", "err", rbErr)
			}
			panic(p)
		case err != nil:
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v: %w", rbErr, err)
			}
		default:
			if cErr := tx.Commit(ctx); cErr != nil {
				err = fmt.Errorf("postgres: commit: %w", cErr)
			}
		}
	}()
	err = txFunc(tx)
	return
}
