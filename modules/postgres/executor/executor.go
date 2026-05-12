package executor

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

/*
This struct should wrap sqlx function calls so that we have decoupling with this 3rd party library,
which will help with mocking and testing.
*/

type Executor interface {
	// dest is singular struct. errors if returns nothing
	GetContext(ctx context.Context, queryerContext sqlx.QueryerContext, dest interface{}, query string, args []interface{}) error
	ExecContext(ctx context.Context, execerContext sqlx.ExecerContext, query string, args []interface{}) (sql.Result, error)
	MustExecContext(ctx context.Context, execerContext sqlx.ExecerContext, query string, args []interface{}) sql.Result
	PreparexContext(ctx context.Context, preparerContext sqlx.PreparerContext, query string) (*sqlx.Stmt, error)
	// dest is a slice of structs. will NOT error when no matches found
	SelectContext(ctx context.Context, queryerContext sqlx.QueryerContext, dest interface{}, query string, args []interface{}) error

	// TODO (Nathaniel.Morihara) Please add functions from sqlx as needed!
}

type ExecutorImpl struct {
}

func NewExecutor() Executor {
	return &ExecutorImpl{}
}

// ExecerContext is an interface used by MustExecContext and LoadFileContext
// Exact same as its sqlx counterpart, defined here so that we can generate a mock for unit testing
//
//go:generate compassmock ExecerContext
type ExecerContext interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// PreparerContext is an interface used by PreparexContext.
// Exact same as its sqlx counterpart, defined here so that we can generate a mock for unit testing
//
//go:generate compassmock PreparerContext
type PreparerContext interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// QueryerContext is an interface used by GetContext and SelectContext
// Exact same as its sqlx counterpart, defined here so that we can generate a mock for unit testing
//
//go:generate compassmock QueryerContext
type QueryerContext interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}

//go:generate compassmock SelectorContext
type SelectorContext interface {
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}
