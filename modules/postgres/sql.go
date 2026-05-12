package postgres

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"compass.com/utils"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx/reflectx"
)

var (
	psql   = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	mapper = reflectx.NewMapper("db")
	// Precompile the regexps.
	regBacktick         = regexp.MustCompile("[`]")
	regSingleQuote      = regexp.MustCompile("[']")
	regPunctuation      = regexp.MustCompile(`([!"#$%&()*+,\-./:;<=>?@\[\\\]^_{|}~])`)
	regInsideWhitespace = regexp.MustCompile(`[\s\p{Zs}]{2,}`)
)

// InsertOptions is used for setting options for
// bulk insert operation
type InsertOptions struct {
	IncludeNilFields bool
	IncludeIDField   bool
	ExcludeFields    []string
}

// UpdateOptions is used for setting options for
// update operation
type UpdateOptions struct {
	IncludeNilFields bool
	ExcludeFields    []string
}

// Select is a wrapper for sq.Select() that uses PostgreSQL $1 syntax.
func Select(columns ...string) sq.SelectBuilder {
	return psql.Select(columns...)
}

// Insert is a wrapper for sq.Insert() that uses PostgreSQL $1 syntax.
func Insert(into string) sq.InsertBuilder {
	return psql.Insert(into)
}

// Update is a wrapper for sq.Update() that uses PostgreSQL $1 syntax.
func Update(table string) sq.UpdateBuilder {
	return psql.Update(table)
}

// Delete is a wrapper for sq.Delete() that uses PostgreSQL $1 syntax.
func Delete(from string) sq.DeleteBuilder {
	return psql.Delete(from)
}

// TODO (Nathaniel.morihara) (DNC-5419) Wrap this in an interface
// ToSQL converts a squirrel Select() or other query builder to an
// SQL string and slice of args. It panics on error, because these
// errors are almost always programming errors.
func ToSQL(sqlizer sq.Sqlizer) (string, []interface{}) {
	query, args, err := sqlizer.ToSql()
	if err != nil {
		panic(fmt.Sprintf("ToSQL error: %v", err))
	}
	return query, args
}

func SliceStructSorted(strct interface{}, includeNilFields bool, includeIdField bool) ([]string, []interface{}) {
	v := reflect.ValueOf(strct)
	keys, values := FieldValueList(v, includeNilFields, includeIdField, nil)

	sort.Sort(&dualSorter{cols: keys, vals: values})
	return keys, values
}

func SliceStructSortedWithExcludes(strct interface{}, includeNilFields bool, includeIdField bool, excludedFields []string) ([]string, []interface{}) {
	v := reflect.ValueOf(strct)
	keys, values := FieldValueList(v, includeNilFields, includeIdField, excludedFields)

	sort.Sort(&dualSorter{cols: keys, vals: values})
	return keys, values
}

// MapStruct converts a struct to a map {field name: field value}
func MapStruct(strct interface{}, includeNilFields bool, includeIdField bool) map[string]interface{} {
	fields := BuildFieldMap(strct)
	pairs := map[string]interface{}{}
	for key := range fields {
		f := fields[key]
		if (includeIdField || key != "id") && (includeNilFields || f.Kind() == reflect.Struct || !f.IsNil()) {
			pairs[key] = f.Interface()
		}
	}
	return pairs
}

// Uses reflection to return a map of all fields in a struct.
func BuildFieldMap(strct interface{}) map[string]reflect.Value {
	v := reflect.ValueOf(strct)
	return FieldMap(v)
}

/*
MapStruct ignores fields named id. We added a param to it that will
include the id field.  This function determines if an id value exists
so that we can determine if we want MapStruct to ignore it or include it.
*/
func IncludeIDFieldInMap(strct interface{}) bool {
	fields := BuildFieldMap(strct)
	if _, ok := fields["id"]; ok {
		return !fields["id"].IsNil()
	}
	return false
}

// Convenience method for BuildModifiedSelectColumns when no modifiers are required.
func BuildSelectColumns(src interface{}) []string {
	return BuildModifiedSelectColumns(src, make(map[string]string))
}

func BuildSelectColumnsWithExcludes(src interface{}, excludes []string) []string {
	selectColumns := BuildSelectColumns(src)
	var notSlice []string
	for _, excludeCol := range excludes {
		notSlice = append(notSlice, fmt.Sprintf("%[1]s as \"%[1]s\"", excludeCol))
	}
	return utils.NewStringSet().InitFromSlice(selectColumns).Not(utils.NewStringSet().InitFromSlice(notSlice)).GetSlice()
}

// Recursively generates the list of columns for a given struct, respecting the db tag.  Tags for embedded structs are
// automatically treated as table aliases.  The modifiers map provides overrides for columns replacing the generated
// column name with the one provided.
// Use as:
//
//	  type Foo struct {
//		   Bar string `db:"bar"
//	  }
//		 cols := postgres.BuildModifiedSelectColumns(&Foo{}, map[string]string{"bar": "bar - 'baz'"})
func BuildModifiedSelectColumns(src interface{}, modifiers map[string]string) []string {
	v := reflect.ValueOf(src)
	fields := FieldList(v)
	var names []string
	for _, key := range fields {
		var name string
		if modifier, ok := modifiers[key]; ok {
			name = modifier
		} else {
			name = key
		}
		names = append(names, fmt.Sprintf("%s as \"%s\"", name, key))
	}
	return names
}

// BuildReturningColumns returns the postgres RETURNING statement for a given struct, respecting the db tag.
func BuildReturningColumns(src interface{}) string {
	cols := BuildSelectColumns(src)
	if len(cols) > 0 {
		return "RETURNING " + strings.Join(cols, ",")
	}
	return ""
}

// BuildReturningColumnsWithExcludes returns the postgres RETURNING statement for a given struct, respecting the db tag.
// Does not include excluded fields
func BuildReturningColumnsWithExcludes(src interface{}, excludedFields []string) string {
	selectColumns := BuildSelectColumnsWithExcludes(src, excludedFields)
	if len(selectColumns) > 0 {
		return "RETURNING " + strings.Join(selectColumns, ",")
	}
	return ""
}

// BuildInsert returns the postgres INSERT statement (a SQL string and a slice of arguments)
// for inserting src as a row in table.
func BuildInsert(table string, src interface{}) (string, []interface{}) {
	cols, vals := SliceStructSorted(src, false, false)
	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix(BuildReturningColumns(src)))
}

// BuildInsertOnConflictDoNothing If conflict for key conflictKey, then do nothing
func BuildInsertOnConflictDoNothing(table string, src interface{}, conflictKey string) (string, []interface{}) {
	cols, vals := SliceStructSorted(src, false, false)
	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix(fmt.Sprintf("ON CONFLICT (%s) DO NOTHING %s", conflictKey, BuildReturningColumns(src))))
}

// BuildInsertWithSuffix Provide custom suffix for the insert query
func BuildInsertWithSuffix(table string, src interface{}, suffix string) (string, []interface{}) {
	cols, vals := SliceStructSorted(src, false, false)
	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix(suffix))
}

// BuildInsertWithId returns the postgres INSERT statement with Id field(a SQL string and a slice of arguments)
// for inserting src as a row in table.
func BuildInsertWithID(table string, src interface{}) (string, []interface{}) {
	var names []string
	var values []interface{}
	pairs := MapStruct(src, true, true)
	for key := range pairs {
		names = append(names, key)
		values = append(values, pairs[key])
	}
	return ToSQL(Insert(table).
		Columns(names...).
		Values(values...).
		Suffix(BuildReturningColumns(src)))
}

// BuildBulkInsert returns the postgres INSERT statement (a SQL string and a slice of arguments)
// for inserting each element in sources as a row in table.
func BuildBulkInsert(table string, sources []interface{}) (string, []interface{}) {
	q := bulkInsertBuilder(table, sources, InsertOptions{})
	return ToSQL(q.Suffix(BuildReturningColumns(sources[0])))
}

// BuildBulkInsertReturning returns the postgres INSERT statement (a SQL string and a slice of arguments)
// for inserting each element in sources as a row in table and returning each inserted element as the returnStruct.
func BuildBulkInsertReturning(table string, sources []interface{}, returnStruct interface{}) (string, []interface{}) {
	q := bulkInsertBuilder(table, sources, InsertOptions{})
	return ToSQL(q.Suffix(BuildReturningColumns(returnStruct)))
}

// BuildBulkInsertWithOptions is similar to BuildBulkInsert with additional options for
// including nil values
func BuildBulkInsertWithOptions(table string, sources []interface{},
	options InsertOptions,
) (string, []interface{}) {
	q := bulkInsertBuilder(table, sources, options)
	return ToSQL(q.Suffix(BuildReturningColumns(sources[0])))
}

func bulkInsertBuilder(table string, sources []interface{},
	options InsertOptions,
) sq.InsertBuilder {
	q := Insert(table)
	cols, _ := SliceStructSortedWithExcludes(sources[0], options.IncludeNilFields, options.IncludeIDField, options.ExcludeFields)
	q = q.Columns(cols...)
	for _, val := range sources {
		_, vals := SliceStructSortedWithExcludes(val, options.IncludeNilFields, options.IncludeIDField, options.ExcludeFields)
		q = q.Values(vals...)
	}
	return q
}

func BuildBulkInsertIgnore(table string, sources []interface{}, conflictKey string) (
	string, []interface{}) {
	q := bulkInsertBuilder(table, sources, InsertOptions{IncludeNilFields: true})
	return ToSQL(q.Suffix(fmt.Sprintf("ON CONFLICT (%s) DO NOTHING %s", conflictKey, BuildReturningColumns(sources[0]))))
}

// Generates the query to find data in a PostgreSQL database
// 'table' is the DB table to upsert to
// 'columns' are the columns in the database to retrieve. BuildSelectColumns() might be useful here
// 'whereClauses' are the clauses that will make up WHERE clause of the query.
//
//	Recommended to use squirrel's "expressions" (squirrel/expr.go),
//	or a map[string]interface{} which will AND key-value pairs together as 'key = value'
//
// Returns a 'SELECT _ FROM _ WHERE _' query as well as the arguments to be plugged into the query
func BuildSelect(
	table string,
	columns []string,
	whereClauses interface{}) (string, []interface{}) {
	// we have to convert map[string]interface to sq.And to force a sort order
	if clauses, ok := whereClauses.(map[string]interface{}); ok {
		cols := make([]string, 0, len(clauses))
		vals := make([]interface{}, 0, len(clauses))
		for k, v := range clauses {
			// we're keeping the slice sorted as we insert values here
			cols = append(cols, k)
			vals = append(vals, v)
		}

		ds := &dualSorter{cols: cols, vals: vals}
		sort.Sort(ds)

		where := sq.And{}
		for i, c := range cols {
			where = append(where, sq.Eq{c: vals[i]})
		}
		whereClauses = where
	}

	return ToSQL(
		Select(columns...).
			From(table).
			Where(whereClauses))
}

func BuildUpdate(table string, src interface{}, id int64, full bool) (string, []interface{}) {
	cols, vals := SliceStructSorted(src, full, false)
	q := Update(table)
	for i, col := range cols {
		q = q.Set(col, vals[i])
	}
	return ToSQL(q.Where(sq.Eq{"id": id}).
		Suffix(BuildReturningColumns(src)))
}

func BuildUpdateWithOptions(table string, src interface{}, id int64, options UpdateOptions) (string, []interface{}) {
	cols, vals := SliceStructSortedWithExcludes(src, options.IncludeNilFields,
		false, options.ExcludeFields)
	q := Update(table)
	for i, col := range cols {
		q = q.Set(col, vals[i])
	}
	var suffix string
	updateCols := BuildSelectColumnsWithExcludes(src, options.ExcludeFields)
	if len(updateCols) > 0 {
		suffix = "RETURNING " + strings.Join(updateCols, ",")
	}
	return ToSQL(q.Where(sq.Eq{"id": id}).
		Suffix(suffix))
}

func BuildUpdateWhere(
	table string,
	src interface{},
	whereClause map[string]interface{},
	includeNilFields bool,
	includeIdFields bool) (string, []interface{}) {
	q := updateBuilderFromSrc(table, src, includeNilFields, includeIdFields)
	return ToSQL(q.Where(whereClause).
		Suffix(BuildReturningColumns(src)))
}

// BuildUpdateWhereNotIncludeNull returns a postgres UPDATE statement excluding rows with
// field values matching the whereNotClause map.
// The statement returned includes NULL values.
// e.g. whereNotClause = {"archived": true} includes rows with archived NULL.
func BuildUpdateWhereNotIncludeNull(
	table string,
	src interface{},
	whereClause map[string]interface{},
	whereNotClause map[string]interface{},
	includeNilFields bool,
	includeIdFields bool,
) (string, []interface{}) {
	return buildUpdateWhereAndWhereNot(table, src, whereClause, whereNotClause,
		includeNilFields, includeIdFields, true)
}

// BuildUpdateWhereNotExcludeNull returns a postgres UPDATE statement excluding rows with
// field values matching the whereNotClause map or NULL.
// e.g. whereNotClause = {"archived": true} excludes rows with archived NULL.
func BuildUpdateWhereNotExcludeNull(
	table string,
	src interface{},
	whereClause map[string]interface{},
	whereNotClause map[string]interface{},
	includeNilFields bool,
	includeIdFields bool,
) (string, []interface{}) {
	return buildUpdateWhereAndWhereNot(table, src, whereClause, whereNotClause,
		includeNilFields, includeIdFields, false)
}

func buildUpdateWhereAndWhereNot(
	table string,
	src interface{},
	whereClause map[string]interface{},
	whereNotClause map[string]interface{},
	includeNilFields bool,
	includeIdFields bool,
	includeNullValues bool) (string, []interface{}) {
	q := updateBuilderFromSrc(table, src, includeNilFields, includeIdFields)
	if len(whereClause) > 0 {
		q = q.Where(whereClause)
	}
	for k, v := range whereNotClause {
		if v == nil || !includeNullValues {
			q = q.Where(sq.NotEq(map[string]interface{}{k: v}))
		} else {
			// Since NotEq does not include NULL values, we explicitly add an OR clause including them when needed.
			// e.g. (WHERE archived <> true OR archived IS NULL)
			q = q.Where(sq.Or{sq.NotEq(map[string]interface{}{k: v}), sq.Eq(map[string]interface{}{k: nil})})
		}
	}
	return ToSQL(q.Suffix(BuildReturningColumns(src)))
}

func updateBuilderFromSrc(table string, src interface{}, includeNilFields bool, includeIdFields bool) sq.UpdateBuilder {
	cols, vals := SliceStructSorted(src, includeNilFields, includeIdFields)
	q := Update(table)
	for i, col := range cols {
		q = q.Set(col, vals[i])
	}
	return q
}

func BuildUpdateWhereWithSetClause(
	table string,
	setClause map[string]interface{},
	whereClause map[string]interface{},
	returnSrc interface{}) (string, []interface{}) {

	return ToSQL(Update(table).
		SetMap(setClause).
		Where(whereClause).
		Suffix(BuildReturningColumns(returnSrc)))
}

// Generates the query to upsert a single struct to a PostgreSQL database
// 'table' is the DB table to upsert to
// 'src' is the struct to be upserted
// 'conflictKey' is the key to be used to determine if there is an already existing row for the struct
//
//	and an UPDATE should occur instead of an INSERT
//
// 'includeNilFields', when true, means that fields that are nil in the struct will be included in the query
// 'includeIdField', when true, means that the struct's 'id' field will be included in the query
// Returns an 'INSERT _ ON CONFLICT UPDATE _' query, as well as the arguments to be plugged into the query.
func BuildUpsert(table string, src interface{}, conflictKey string) (string, []interface{}) {
	options := InsertOptions{
		IncludeNilFields: true,
		IncludeIDField:   IncludeIDFieldInMap(src),
	}
	return buildUpsert(table, src, conflictKey, options)
}

// Generates the upsert query for a single record with the ability to set additional options
func BuildUpsertWithOptions(table string, src interface{}, conflictKey string,
	options InsertOptions,
) (string, []interface{}) {
	return buildUpsert(table, src, conflictKey, options)
}

func buildUpsert(table string, src interface{}, conflictKey string,
	options InsertOptions,
) (string, []interface{}) {
	var cols []string
	var vals []interface{}
	var returningStatement string
	if len(options.ExcludeFields) > 0 {
		cols, vals = SliceStructSortedWithExcludes(src, options.IncludeNilFields,
			options.IncludeIDField, options.ExcludeFields)
		returningStatement = BuildReturningColumnsWithExcludes(src, options.ExcludeFields)
	} else {
		cols, vals = SliceStructSorted(src, options.IncludeNilFields, options.IncludeIDField)
		returningStatement = BuildReturningColumns(src)
	}
	var updateParts []string
	for _, key := range cols {
		updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", key, key))
	}

	return ToSQL(Insert(table).
		Columns(cols...).
		Values(vals...).
		Suffix(fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s %s",
			conflictKey,
			strings.Join(updateParts, ", "),
			returningStatement)))
}

func BuildBulkUpsertWithNils(table string, sources []interface{}, conflictKey string) (string, []interface{}) {
	return BuildBulkUpsert(table, sources, conflictKey, true, false)
}

// Generates the query to upsert a slice of structs to a PostgreSQL database
// 'table' is the DB table to upsert to
// 'sources' are the structs to be upserted
// 'conflictKey' is the key to be used to determine if there is an already existing row for the struct
//
//	and an UPDATE should occur instead of an INSERT
//
// 'includeNilFields', when true, means that fields that are nil in the struct will be included in the query
// 'includeIdField', when true, means that the struct's 'id' field will be included in the query
// Returns an 'INSERT _ ON CONFLICT UPDATE _' query, as well as the arguments to be plugged into the query.
func BuildBulkUpsert(
	table string,
	sources []interface{},
	conflictKey string,
	includeNilFields bool,
	includeIdField bool) (string, []interface{}) {
	q := Insert(table)
	var updateParts []string
	cols, _ := SliceStructSorted(sources[0], includeNilFields, includeIdField)
	for _, key := range cols {
		updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", key, key))
	}
	q = q.Columns(cols...)
	for _, val := range sources {
		_, vals := SliceStructSorted(val, includeNilFields, includeIdField)
		q = q.Values(vals...)
	}
	return ToSQL(q.
		Suffix(fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s %s",
			conflictKey, strings.Join(updateParts, ", "), BuildReturningColumns(sources[0]))))
}

type UpsertCustomizer interface {
	CreateOperation(string, string) string
}
type UpsertIncrement struct{}

func (i UpsertIncrement) CreateOperation(columnName string, tableName string) string {
	return fmt.Sprintf("%s = %s.%s + EXCLUDED.%s", columnName, tableName, columnName, columnName)
}

// Allows for doing an upsert with custom operations on particular columns (for example, increment by a given amount)
// customOperations is a map of column name -> custom operation.
func BuildBulkUpsertWithCustomOperations(
	table string,
	sources []interface{},
	conflictKey string,
	customOperations map[string]UpsertCustomizer,
	includeNilFields bool,
	includeIdField bool,
) (string, []interface{}) {
	q := Insert(table)
	var updateParts []string
	cols, _ := SliceStructSorted(sources[0], includeNilFields, includeIdField)
	for _, key := range cols {
		customOperationCreator, ok := customOperations[key]
		if ok {
			updateParts = append(updateParts, customOperationCreator.CreateOperation(key, table))
		} else {
			updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", key, key))
		}
	}
	q = q.Columns(cols...)
	for _, val := range sources {
		_, vals := SliceStructSorted(val, includeNilFields, includeIdField)
		q = q.Values(vals...)
	}
	return ToSQL(q.
		Suffix(fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s %s",
			conflictKey, strings.Join(updateParts, ", "), BuildReturningColumns(sources[0]))))
}

func BuildDelete(table string, id int64) (string, []interface{}) {
	return ToSQL(Delete(table).Where(sq.Eq{"id": id}))
}

func BuildBulkDelete(table string, ids []int64) (string, []interface{}) {
	return ToSQL(Delete(table).Where(sq.Eq{"id": ids}))
}

// Returns a string slice of the columns for squirrel's OrderBy functions to consume.
// Appends either " ASC" or " DESC" to the last column string (since that's what squirrel requires)
func BuildOrderBy(columns []string, isAsc bool) []string {
	if len(columns) > 0 {
		lastIndex := len(columns) - 1
		if isAsc {
			columns[lastIndex] = fmt.Sprintf("%s %s", columns[lastIndex], "ASC")
		} else {
			columns[lastIndex] = fmt.Sprintf("%s %s", columns[lastIndex], "DESC")
		}
	}
	return columns
}

// TextStripToWords removes duplicate whitespaces and escapes all punctuations.
func TextStripToWords(input string) string {
	output := strings.TrimSpace(input)
	output = regPunctuation.ReplaceAllString(output, "\\${1}")
	output = regSingleQuote.ReplaceAllString(output, "''")
	output = regInsideWhitespace.ReplaceAllLiteralString(output, " ")
	output = regBacktick.ReplaceAllLiteralString(output, "")
	return output
}

// TextStripPunctuationToWords escapes punctuation chars.
// squirrel already escapes backtick, singleQuote, and whitespace inside
// however, not all punctuations are escaped properly, example: *
func TextStripPunctuationToWords(input string) string {
	output := strings.TrimSpace(input)
	output = regPunctuation.ReplaceAllString(output, "\\${1}")
	return output
}

// TextQueryToSq creates a squirrel of POSIX regex expressions that search for any of the words
// in the provided textQuery in any of the columns provided as keys in the map of columnsToRegexF
// using the regex format provided as the column's map value. With multiple query it AND these ORs
// to include all search terms .Bindvars are used for word values to
// prevent SQL injection and POSIX regex command characters are stripped.
// Example SQL:
// ((p.first_name ~* ('^token1') OR p.last_name ~* ('^token1') OR p.email ~* ('token1'))
// AND (p.first_name ~* ('^token2') OR p.last_name ~* ('^token2') OR p.email ~* ('token2')))
func TextQueryToSq(textQuery string, columnsToRegexF map[string]string) sq.Sqlizer {
	stripped := TextStripPunctuationToWords(textQuery)
	words := strings.Fields(stripped)
	// Create a bindvar ? placeholder for the word values to protect against SQL injection.
	wordPlaceholders := "||?||"
	// Squirrel expects values/args to be an array of interface.
	// Make one and populate it with the word values.
	wordsValues := make([]interface{}, len(words))
	for index := range words {
		wordsValues[index] = words[index]
	}
	// Compose a final POSIX regex expression for each column and OR them using sq.
	// For multiple tokens, AND these ORs.
	ands := sq.And{}
	for i := range words {
		ors := sq.Or{}
		for column, regexF := range columnsToRegexF {
			// Insert the regex fragment of bindvar placeholders into the regex format for
			// this column.
			regex := fmt.Sprintf(regexF, wordPlaceholders)
			// Provide the SQL expression with bindvar placeholders along with the words values.
			ors = append(ors, sq.Expr(fmt.Sprintf("%s ~* (%s)", column, regex), wordsValues[i]))
		}
		ands = append(ands, ors)
	}
	return ands
}

func TextQueryToSqlString(textQuery string, columnsToRegexF map[string]string) string {
	stripped := TextStripToWords(textQuery)
	words := strings.Fields(stripped)
	wordsValues := make([]interface{}, len(words))
	for index := range words {
		wordsValues[index] = words[index]
	}
	ands := []string{}
	for i := range words {
		ors := []string{}
		for column, regexF := range columnsToRegexF {
			regex := fmt.Sprintf(regexF, wordsValues[i])
			ors = append(ors, fmt.Sprintf("%s ~* %s", column, regex))
		}
		ands = append(ands, strings.Join(ors, " OR "))
	}

	return strings.Join(ands, " AND ")
}

func FieldList(v reflect.Value) []string {
	v = reflect.Indirect(v)
	if k := v.Kind(); k != reflect.Struct {
		panic("expecting struct")
	}
	tm := mapper.TypeMap(v.Type())
	fields := make([]string, 0, len(tm.Names))
	for tagName := range tm.Names {
		fields = append(fields, tagName)
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i] <= fields[j]
	})
	return fields
}

func FieldMap(v reflect.Value) map[string]reflect.Value {
	v = reflect.Indirect(v)
	if k := v.Kind(); k != reflect.Struct {
		panic("expecting struct")
	}
	r := map[string]reflect.Value{}
	tm := mapper.TypeMap(v.Type())
	for tagName, fi := range tm.Names {
		r[tagName] = reflectx.FieldByIndexesReadOnly(v, fi.Index)
	}
	return r
}

func FieldValueList(v reflect.Value, includeNilFields bool, includeIdField bool, excludedFields []string) ([]string, []interface{}) {
	v = reflect.Indirect(v)
	if k := v.Kind(); k != reflect.Struct {
		panic("expecting struct")
	}
	tm := mapper.TypeMap(v.Type())
	fields := []string{}
	values := []interface{}{}
	excludedFieldsSet := utils.NewStringSet().InitFromSlice(excludedFields)
	for tagName, fi := range tm.Names {
		if !includeIdField && tagName == "id" {
			continue
		}
		if excludedFieldsSet.Contains(tagName) {
			continue
		}
		val := reflectx.FieldByIndexesReadOnly(v, fi.Index)
		if !includeNilFields && val.Kind() != reflect.Struct && val.IsNil() {
			continue
		}
		fields = append(fields, tagName)
		values = append(values, val.Interface())
	}
	return fields, values
}

type dualSorter struct {
	cols []string
	vals []interface{}
}

func (d *dualSorter) Len() int {
	return len(d.cols)
}

func (d *dualSorter) Less(i, j int) bool {
	return d.cols[i] <= d.cols[j]
}

func (d *dualSorter) Swap(i, j int) {
	d.cols[i], d.cols[j] = d.cols[j], d.cols[i]
	d.vals[i], d.vals[j] = d.vals[j], d.vals[i]
}
