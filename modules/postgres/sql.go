package postgres

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

// Builder is the package-level squirrel statement builder, pre-configured
// with PostgreSQL's `$N` placeholder format.
var Builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// Select starts a SELECT statement with the supplied columns.
func Select(columns ...string) sq.SelectBuilder { return Builder.Select(columns...) }

// Insert starts an INSERT statement into the supplied table.
func Insert(into string) sq.InsertBuilder { return Builder.Insert(into) }

// Update starts an UPDATE statement on the supplied table.
func Update(table string) sq.UpdateBuilder { return Builder.Update(table) }

// Delete starts a DELETE statement from the supplied table.
func Delete(from string) sq.DeleteBuilder { return Builder.Delete(from) }

// ToSQL is a convenience that compiles a Sqlizer and returns (query, args).
// It returns an error rather than panicking so callers can decide how to
// surface the failure.
func ToSQL(s sq.Sqlizer) (string, []any, error) {
	q, args, err := s.ToSql()
	if err != nil {
		return "", nil, fmt.Errorf("postgres: build sql: %w", err)
	}
	return q, args, nil
}
