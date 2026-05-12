package expression

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"database/sql/driver"
	"github.com/lib/pq"

	sq "github.com/Masterminds/squirrel"
)

// Squirrel Sqlizers for Boolean Postgres Array Operators
//
//	See: https://www.postgresql.org/docs/13/functions-array.html
type ArrayOverlap map[string]interface{}
type ArrayContains map[string]interface{}
type ArrayContainedBy map[string]interface{}

var _ sq.Sqlizer = ArrayOverlap{}
var _ sq.Sqlizer = ArrayContains{}
var _ sq.Sqlizer = ArrayContainedBy{}

func (a ArrayContains) ToSql() (sql string, args []interface{}, err error) {
	return asArrayOpSql(a, "@>")
}

func (a ArrayContainedBy) ToSql() (sql string, args []interface{}, err error) {
	return asArrayOpSql(a, "<@")
}

func (a ArrayOverlap) ToSql() (sql string, args []interface{}, err error) {
	return asArrayOpSql(a, "&&")
}

func asArrayOpSql(expressionFields map[string]interface{}, operator string) (sql string, args []interface{}, err error) {
	keyExpressions := make([]string, 0, len(expressionFields))
	for _, key := range getSortedKeys(expressionFields) {
		value, err := asArrayValue(expressionFields[key])
		if err != nil {
			return "", nil, err
		}
		args = append(args, value)
		keyExpressions = append(keyExpressions, fmt.Sprintf("%s %s ?", key, operator))
	}
	return strings.Join(keyExpressions, " AND "), args, nil
}

func asArrayValue(value interface{}) (interface{}, error) {
	// Values that implement valuer (pq arrays for example) should be allowed to provide their own argument values
	if isValuer(value) {
		return value, nil
	}
	//Otherwise, values must be valid lists that can be handled as Postgres Array
	if !isListType(value) {
		return nil, fmt.Errorf("must use array or slice with array operators")
	}

	// Allow pq.Array to handle value determination
	return pq.Array(value), nil
}

func isValuer(value interface{}) bool {
	if _, ok := value.(driver.Valuer); ok {
		return true
	}
	if _, ok := value.(*driver.Valuer); ok {
		return true
	}
	return false
}

// Lifted from Masterminds/squirrel expr.go
func isListType(val interface{}) bool {
	if driver.IsValue(val) {
		return false
	}
	valVal := reflect.ValueOf(val)
	return valVal.Kind() == reflect.Array || valVal.Kind() == reflect.Slice
}

// Lifted from Masterminds/squirrel expr.go
func getSortedKeys(exp map[string]interface{}) []string {
	sortedKeys := make([]string, 0, len(exp))
	for k := range exp {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	return sortedKeys
}
