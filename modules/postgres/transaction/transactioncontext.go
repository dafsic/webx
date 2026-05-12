package transaction

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

//go:generate compassmock TransactionContext
type TransactionContext interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result
	PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Rebind(query string) string
	sqlx.QueryerContext
	sqlx.PreparerContext
	sqlx.ExecerContext
}

var _ TransactionContext = &sqlx.Tx{}
