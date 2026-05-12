package executor

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// TODO (Nathaniel.Morihara) Please add implementations as needed!

func (q *ExecutorImpl) GetContext(
	ctx context.Context,
	queryerContext sqlx.QueryerContext,
	dest interface{},
	query string,
	args []interface{},
) (
	err error,
) {
	return sqlx.GetContext(ctx, queryerContext, dest, query, args...)
}

func (q *ExecutorImpl) ExecContext(
	ctx context.Context,
	execerContext sqlx.ExecerContext,
	query string,
	args []interface{},
) (
	sqlResult sql.Result,
	err error,
) {
	return execerContext.ExecContext(ctx, query, args...)
}

func (q *ExecutorImpl) MustExecContext(
	ctx context.Context,
	execerContext sqlx.ExecerContext,
	query string,
	args []interface{},
) (
	sqlResult sql.Result,
) {
	return sqlx.MustExecContext(ctx, execerContext, query, args...)
}

func (q *ExecutorImpl) PreparexContext(
	ctx context.Context,
	preparerContext sqlx.PreparerContext,
	query string,
) (
	*sqlx.Stmt,
	error,
) {
	return sqlx.PreparexContext(ctx, preparerContext, query)
}

func (q *ExecutorImpl) SelectContext(
	ctx context.Context,
	queryerContext sqlx.QueryerContext,
	dest interface{},
	query string,
	args []interface{},
) (
	err error,
) {
	return sqlx.SelectContext(ctx, queryerContext, dest, query, args...)
}
