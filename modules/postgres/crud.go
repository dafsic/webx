package postgres

import (
	"reflect"
	"sort"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx/reflectx"
)

// mapper resolves struct fields by their `db` tag, matching the tag used by
// pgx's RowToStructByName when scanning rows back into structs.
var mapper = reflectx.NewMapper("db")

// StructOptions controls how a struct is decomposed into columns and values.
type StructOptions struct {
	// IncludeNil includes fields whose (nilable) value is nil. By default nil
	// fields are skipped so the database default / existing value is kept.
	IncludeNil bool
	// IncludeID includes the "id" column. By default "id" is skipped so the
	// database can assign a serial / generated primary key.
	IncludeID bool
	// Exclude lists db-tag names to leave out entirely.
	Exclude []string
}

// StructColumns reflects over src (a struct or pointer to struct) and returns
// its db-tagged column names together with the matching values, sorted by
// column name for deterministic SQL. nil and "id" handling follow opts.
func StructColumns(src any, opts StructOptions) (cols []string, vals []any) {
	v := reflect.Indirect(reflect.ValueOf(src))
	if v.Kind() != reflect.Struct {
		panic("postgres: StructColumns expects a struct")
	}

	excluded := make(map[string]struct{}, len(opts.Exclude))
	for _, e := range opts.Exclude {
		excluded[e] = struct{}{}
	}

	tm := mapper.TypeMap(v.Type())
	cols = make([]string, 0, len(tm.Names))
	for name := range tm.Names {
		if name == "id" && !opts.IncludeID {
			continue
		}
		if _, skip := excluded[name]; skip {
			continue
		}
		cols = append(cols, name)
	}
	sort.Strings(cols)

	vals = make([]any, 0, len(cols))
	kept := cols[:0]
	for _, name := range cols {
		fv := reflectx.FieldByIndexesReadOnly(v, tm.Names[name].Index)
		if !opts.IncludeNil && isNil(fv) {
			continue
		}
		kept = append(kept, name)
		vals = append(vals, fv.Interface())
	}
	return kept, vals
}

// isNil reports whether v holds a nil value, guarding against the panic that
// reflect.Value.IsNil triggers on non-nilable kinds.
func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// SelectColumns returns the sorted db-tagged column names of src, suitable for
// a SELECT or RETURNING list. The "id" column is included.
func SelectColumns(src any) []string {
	cols, _ := StructColumns(src, StructOptions{IncludeNil: true, IncludeID: true})
	return cols
}

// BuildInsert builds an INSERT for src into table, returning the inserted row
// via RETURNING. nil fields and "id" are skipped so generated columns are
// populated by the database.
func BuildInsert(table string, src any) (string, []any, error) {
	cols, vals := StructColumns(src, StructOptions{})
	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix("RETURNING " + columnList(SelectColumns(src))))
}

// BuildUpdate builds an UPDATE on table setting src's non-nil columns where the
// where clause matches, returning the updated row. "id" is never written.
func BuildUpdate(table string, src any, where sq.Sqlizer) (string, []any, error) {
	cols, vals := StructColumns(src, StructOptions{})
	q := Update(table)
	for i, c := range cols {
		q = q.Set(c, vals[i])
	}
	return ToSQL(q.
		Where(where).
		Suffix("RETURNING " + columnList(SelectColumns(src))))
}

// BuildUpsert builds an INSERT ... ON CONFLICT (conflictKey) DO UPDATE for src,
// returning the resulting row. All non-nil columns are inserted and, on
// conflict, overwritten with the excluded (proposed) values.
func BuildUpsert(table string, src any, conflictKey string) (string, []any, error) {
	cols, vals := StructColumns(src, StructOptions{IncludeID: includeID(src)})
	set := make([]string, len(cols))
	for i, c := range cols {
		set[i] = c + " = EXCLUDED." + c
	}
	suffix := "ON CONFLICT (" + conflictKey + ") DO UPDATE SET " +
		columnList(set) + " RETURNING " + columnList(SelectColumns(src))
	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix(suffix))
}

// BuildSelect builds a SELECT of src's columns from table filtered by where.
func BuildSelect(table string, src any, where sq.Sqlizer) (string, []any, error) {
	return ToSQL(Select(SelectColumns(src)...).
		From(table).
		Where(where))
}

// BuildDelete builds a DELETE from table filtered by where.
func BuildDelete(table string, where sq.Sqlizer) (string, []any, error) {
	return ToSQL(Delete(table).Where(where))
}

// includeID reports whether src carries a non-nil "id" value, so an upsert can
// target an existing row by its primary key when one is supplied.
func includeID(src any) bool {
	v := reflect.Indirect(reflect.ValueOf(src))
	tm := mapper.TypeMap(v.Type())
	fi, ok := tm.Names["id"]
	if !ok {
		return false
	}
	return !isNil(reflectx.FieldByIndexesReadOnly(v, fi.Index))
}

// columnList joins columns with ", ".
func columnList(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}
