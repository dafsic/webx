package postgres

import (
	"strings"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"compass.com/utils/pointer"
)

type SqlSuite struct {
	suite.Suite
}

func (s *SqlSuite) SetupSuite() {
}

func (s *SqlSuite) TestTextStripToWords() {
	textQuery := "k[e#v%i$|n d`;<a>:n / : b&&,=e!!!n c.\\a\"m]e&r-o?n m{c}c||lu%r~e@"
	expected := `k\[e\#v\%i\$\|n d\;\<a\>\:n \/ \: b\&\&\,\=e\!\!\!n c\.\\a\"m\]e\&r\-o\?n m\{c\}c\|\|lu\%r\~e\@`
	stripped := TextStripToWords(textQuery)
	require.Equal(s.T(), expected, stripped)
}

func (s *SqlSuite) TextStripPunctuationToWords() {
	textQuery := "k[e#v%i$|n d`;<a>:n / : b&&,=e!!!n c.\\a\"m]e&r-o?n m{c}c||lu%r~e@"
	expected := `k\[e\#v\%i\$\|n d\;\<a\>\:n \/ \: b\&\&\,\=e\!\!\!n c\.\\a\"m\]e\&r\-o\?n m\{c\}c\|\|lu\%r\~e\@`
	stripped := TextStripToWords(textQuery)
	require.Equal(s.T(), expected, stripped)
}

func (s *SqlSuite) TestTextStripToWordsEscapeQuote() {
	textQuery := "Deal's Query"
	expected := "Deal''s Query"
	stripped := TextStripToWords(textQuery)
	require.Equal(s.T(), expected, stripped)
}

func (s *SqlSuite) TestBuildSelect_WhenColumnsStrings_ReturnsQueryWithArgs() {
	// Arrange
	table := "Table"
	columns := []string{"id"}
	whereClauses := squirrel.Eq{"id": pointer.String("123")}

	// Act
	query, args := BuildSelect(table, columns, whereClauses)

	// Assert
	require.True(s.T(), strings.Contains(query, "SELECT id FROM Table WHERE id = $1"))
	require.Equal(s.T(), []interface{}{"123"}, args)
}

func (s *SqlSuite) TestBuildSelect_WhenColumnsFromModel_ReturnsQueryWithArgs() {
	// Arrange
	table := "Table"
	columns := BuildSelectColumns(&rowStructForTests{})
	whereClauses := squirrel.Eq{"id": pointer.String("123")}

	// Act
	query, args := BuildSelect(table, columns, whereClauses)

	// Assert
	require.Equal(s.T(), "SELECT field as \"field\", id as \"id\" FROM Table WHERE id = $1", query)
	require.Equal(s.T(), []interface{}{"123"}, args)
}

func (s *SqlSuite) TestBuildSelectWithExcludesRemovesExcludedFields() {
	// Arrange
	table := "Table"
	columns := BuildSelectColumnsWithExcludes(&rowStructForTests{}, []string{"field"})
	whereClauses := squirrel.Eq{"id": pointer.String("123")}

	// Act
	query, args := BuildSelect(table, columns, whereClauses)

	// Assert
	require.Equal(s.T(), "SELECT id as \"id\" FROM Table WHERE id = $1", query)
	require.Equal(s.T(), []interface{}{"123"}, args)
}

func (s *SqlSuite) TestBuildSelect_WhenWhereClausesExpressions_ReturnsQueryWithArgs() {
	// Arrange
	table := "Table"
	columns := BuildSelectColumns(&rowStructForTests{})
	whereClauses := squirrel.And{
		squirrel.Or{
			squirrel.Eq{
				"id": pointer.String("123"),
			},
			squirrel.Eq{
				"id": pointer.String("456"),
			},
		},
		squirrel.Eq{
			"field": pointer.String("Some Value"),
		},
	}

	// Act
	query, args := BuildSelect(table, columns, whereClauses)

	// Assert
	require.Equal(s.T(), "SELECT field as \"field\", id as \"id\" FROM Table WHERE ((id = $1 OR id = $2) AND field = $3)", query)
	require.Equal(s.T(), []interface{}{"123", "456", "Some Value"}, args)
}

func (s *SqlSuite) TestBuildSelect_WhenWhereClausesHasSlicesAndArrays_ReturnsQueryWithArgs() {
	// Arrange
	type slicesAndArraysForTest struct {
		Slice []*string      `db:"slice"`
		Array pq.StringArray `db:"array"`
	}
	table := "Table"
	columns := BuildSelectColumns(&slicesAndArraysForTest{})

	whereClauses := MapStruct(
		&slicesAndArraysForTest{
			Slice: []*string{pointer.String("123"), pointer.String("456")},
			Array: pq.StringArray{"abc", "def"},
		},
		false,
		false)

	// Act
	query, args := BuildSelect(table, columns, whereClauses)

	// Assert
	require.True(s.T(), strings.Contains(query, "SELECT"))
	require.True(s.T(), strings.Contains(query, "slice as \"slice\""))
	require.True(s.T(), strings.Contains(query, "array as \"array\""))
	require.True(s.T(), strings.Contains(query, "FROM Table WHERE"))
	require.True(s.T(), strings.Contains(query, "array = $"))
	require.True(s.T(), strings.Contains(query, "slice IN ($"))

	require.True(s.T(), containsString(args, "{\"abc\",\"def\"}"))
	require.True(s.T(), contains(args, pointer.String("123")))
	require.True(s.T(), contains(args, pointer.String("456")))
}

func (s *SqlSuite) TestBuildSelect_WhenWhereClausesMap_ReturnsQueryWithArgs() {
	// Arrange
	table := "Table"
	model := BuildSelectColumns(&rowStructForTests{})
	whereClauses := make(map[string]interface{})
	whereClauses["id"] = pointer.String("123")
	whereClauses["field"] = pointer.String("Some Value")

	// Act
	query, args := BuildSelect(table, model, whereClauses)

	// Assert
	require.Equal(s.T(), "SELECT field as \"field\", id as \"id\" FROM Table WHERE (field = $1 AND id = $2)", query)
	require.Equal(s.T(), []interface{}{"Some Value", "123"}, args)
}

func (s *SqlSuite) TestBuildBulkUpsert_WhenCalled_ReturnsQueryWithoutNilsOrIdFields() {
	// Arrange
	table := "Table"
	sources := []interface{}{
		rowStructForTests{
			Id:    pointer.String("123"),
			Field: nil,
		},
	}
	conflictKey := "id"
	IncludeNilFields := false
	includeIdField := false

	// Act
	query, args := BuildBulkUpsert(table, sources, conflictKey, IncludeNilFields, includeIdField)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table VALUES () ON CONFLICT (id) DO UPDATE SET  RETURNING field as \"field\",id as \"id\"", query)
	require.Nil(s.T(), args)
}

func (s *SqlSuite) TestBuildBulkUpsert_WhenIncludeNils_ReturnsQueryWithNils() {
	// Arrange
	table := "Table"
	sources := []interface{}{
		rowStructForTests{
			Id:    pointer.String("123"),
			Field: nil,
		},
	}
	conflictKey := "id"
	IncludeNilFields := true
	includeIdField := false

	// Act
	query, args := BuildBulkUpsert(table, sources, conflictKey, IncludeNilFields, includeIdField)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table (field) VALUES ($1) ON CONFLICT (id) DO UPDATE SET field = EXCLUDED.field RETURNING field as \"field\",id as \"id\"", query)
	var emptyStrPtr *string
	require.Equal(s.T(), []interface{}{emptyStrPtr}, args)
}

func (s *SqlSuite) TestBuildBulkUpsert_WhenIncludeIdFields_ReturnsQueryWithIdFields() {
	// Arrange
	table := "Table"
	sources := []interface{}{
		rowStructForTests{
			Id:    pointer.String("123"),
			Field: nil,
		},
	}
	conflictKey := "id"
	IncludeNilFields := false
	includeIdField := true

	// Act
	query, args := BuildBulkUpsert(table, sources, conflictKey, IncludeNilFields, includeIdField)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table (id) VALUES ($1) ON CONFLICT (id) DO UPDATE SET id = EXCLUDED.id RETURNING field as \"field\",id as \"id\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("123")}, args)
}

func (s *SqlSuite) TestBuildBulkUpsert_WhenIncludeNilAndIdFields_ReturnsQueryWithNilAndIdFields() {
	// Arrange
	table := "Table"
	sources := []interface{}{
		rowStructForTests{
			Id:    pointer.String("123"),
			Field: nil,
		},
	}
	conflictKey := "id"
	IncludeNilFields := true
	includeIdField := true

	// Act
	query, args := BuildBulkUpsert(table, sources, conflictKey, IncludeNilFields, includeIdField)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table (field,id) VALUES ($1,$2) ON CONFLICT (id) DO UPDATE SET field = EXCLUDED.field, id = EXCLUDED.id RETURNING field as \"field\",id as \"id\"", query)
	var emptyStrPtr *string
	require.Equal(s.T(), []interface{}{emptyStrPtr, pointer.String("123")}, args)
}

func (s *SqlSuite) TestBuildBulkUpsertWithNils_WhenCalled_ReturnsQueryWithNils() {
	// Arrange
	table := "Table"
	sources := []interface{}{
		rowStructForTests{
			Id:    pointer.String("123"),
			Field: nil,
		},
	}
	conflictKey := "id"

	// Act
	query, args := BuildBulkUpsertWithNils(table, sources, conflictKey)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table (field) VALUES ($1) ON CONFLICT (id) DO UPDATE SET field = EXCLUDED.field RETURNING field as \"field\",id as \"id\"", query)
	var emptyStrPtr *string
	require.Equal(s.T(), []interface{}{emptyStrPtr}, args)
}

func (s *SqlSuite) TestBuildBulkUpsertWithCustomOperations_WithIncrement() {
	// Arrange
	table := "Table"
	field := 1
	sources := []interface{}{
		rowStructIntegerForTests{
			Id:    pointer.String("123"),
			Field: &field,
		},
	}
	conflictKey := "id"
	customOperationMap := map[string]UpsertCustomizer{"field": UpsertIncrement{}}

	// Act
	query, args := BuildBulkUpsertWithCustomOperations(table, sources, conflictKey, customOperationMap, false, true)

	// Assert
	require.Equal(s.T(), "INSERT INTO Table (field,id) VALUES ($1,$2) ON CONFLICT (id) DO UPDATE SET field = Table.field + EXCLUDED.field, id = EXCLUDED.id RETURNING field as \"field\",id as \"id\"", query)
	require.Equal(s.T(), []interface{}{pointer.Int(1), pointer.String("123")}, args)
}

func (s *SqlSuite) TestBuildOrderBy_WhenAsc_ShouldReturnQueryWithAsc() {
	// Arrange
	columns := []string{"column1", "column2"}
	isAsc := true

	// Act
	result := BuildOrderBy(columns, isAsc)

	// Assert
	require.Equal(s.T(), 2, len(result))
	require.Equal(s.T(), "column1", result[0])
	require.Equal(s.T(), "column2 ASC", result[1])
}

func (s *SqlSuite) TestBuildOrderBy_WhenDesc_ShouldReturnQueryWithAsc() {
	// Arrange
	columns := []string{"column1", "column2"}
	isAsc := false

	// Act
	result := BuildOrderBy(columns, isAsc)

	// Assert
	require.Equal(s.T(), 2, len(result))
	require.Equal(s.T(), "column1", result[0])
	require.Equal(s.T(), "column2 DESC", result[1])
}

func (s *SqlSuite) TestBuildOrderBy_WhenEmptySlice_ShouldReturnEmptySlice() {
	// Arrange
	columns := []string{}
	isAsc := true

	// Act
	result := BuildOrderBy(columns, isAsc)

	// Assert
	require.Equal(s.T(), 0, len(result))
}

func (s *SqlSuite) TestBuildInsert_FullQueryStringMatch() {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	query, args := BuildInsert("test", tt)
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3) RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("25"), pointer.String("email@email.com"), pointer.String("test")}, args)
}

func (s *SqlSuite) TestBuildInsertIgnore() {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	query, args := BuildInsertOnConflictDoNothing("test", tt, "email")
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("25"), pointer.String("email@email.com"), pointer.String("test")}, args)
}

func (s *SqlSuite) TestBuildUpdate_TwoFields() {
	tt := &rowStructForTests{pointer.String("1"), pointer.String("field")}
	query, args := BuildUpdate("test", tt, 1, true)
	require.Equal(s.T(), "UPDATE test SET field = $1 WHERE id = $2 RETURNING field as \"field\",id as \"id\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("field"), int64(1)}, args)
}

func (s *SqlSuite) TestBuildUpdateWithOptions() {
	tt := &row2StructForTests{pointer.String("name"), pointer.String("age"), pointer.String("email")}
	query, args := BuildUpdateWithOptions("test", tt, 1, UpdateOptions{IncludeNilFields: true, ExcludeFields: []string{"age"}})
	require.Contains(s.T(), query, "UPDATE test SET email = $1, name = $2 WHERE id = $3")
	require.Contains(s.T(), query, "email as \"email\"")
	require.Contains(s.T(), query, "name as \"name\"")
	require.NotContains(s.T(), query, "age as \"age\"")
	require.Equal(s.T(), []interface{}{pointer.String("email"), pointer.String("name"), int64(1)}, args)
}

func (s *SqlSuite) TestBuildUpdate_ThreeFields_NoIncludeNil() {
	tt := &row2StructForTests{Name: pointer.String("name"), Age: pointer.String("25")}
	query, args := BuildUpdate("test", tt, 1, false)
	require.Equal(s.T(), "UPDATE test SET age = $1, name = $2 WHERE id = $3 RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("25"), pointer.String("name"), int64(1)}, args)
}

func (s *SqlSuite) TestBuildUpdateWhere_WhenCalled_ShouldReturnColumnsAndValues() {
	// Arrange
	name := "My Name"
	age := "My Age"
	email := "My Email"
	tableName := "TableName"
	src := row2StructForTests{
		Name:  &name,
		Age:   &age,
		Email: &email,
	}
	whereClause := squirrel.Eq{"email": email}

	// Act
	query, args := BuildUpdateWhere(tableName, src, whereClause, false, false)

	// Assert
	expectedQuery := "UPDATE TableName " +
		"SET age = $1, email = $2, name = $3 " +
		"WHERE email = $4 " +
		"RETURNING age as \"age\",email as \"email\",name as \"name\""
	expectedArgs := []interface{}{&age, &email, &name, email}
	require.Equal(s.T(), expectedQuery, query)
	require.Equal(s.T(), expectedArgs, args)
}

func (s *SqlSuite) TestBuildUpdateWhereNot_WhenCalled_ShouldReturnColumnsAndValues() {
	// Arrange
	name := "My Name"
	age := "My Age"
	email := "My Email"
	tableName := "TableName"
	src := row2StructForTests{
		Name:  &name,
		Age:   &age,
		Email: &email,
	}

	testCases := []struct {
		name        string
		where       map[string]interface{}
		whereNot    map[string]interface{}
		excludeNull bool
		wantQuery   []string
		wantArgs    []interface{}
	}{
		{
			name:     "no where clause",
			where:    nil,
			whereNot: map[string]interface{}{"age": age},
			wantQuery: []string{"UPDATE TableName " +
				"SET age = $1, email = $2, name = $3 WHERE " +
				"(age <> $4 OR age IS NULL) ",
				"RETURNING age as \"age\",email as \"email\",name as \"name\""},
			wantArgs: []interface{}{&age, &email, &name, age},
		},
		{
			name:     "one where not clause",
			where:    map[string]interface{}{"email": email},
			whereNot: map[string]interface{}{"name": nil},
			wantQuery: []string{"UPDATE TableName " +
				"SET age = $1, email = $2, name = $3 " +
				"WHERE email = $4 AND name IS NOT NULL " +
				"RETURNING age as \"age\",email as \"email\",name as \"name\""},
			wantArgs: []interface{}{&age, &email, &name, email},
		},
		{
			name:     "multiple where not clauses",
			where:    map[string]interface{}{"email": email},
			whereNot: map[string]interface{}{"name": nil, "age": age},
			wantQuery: []string{"UPDATE TableName " +
				"SET age = $1, email = $2, name = $3 WHERE email = $4 AND ",
				" name IS NOT NULL ", " (age <> $5 OR age IS NULL) ",
				"RETURNING age as \"age\",email as \"email\",name as \"name\""},
			wantArgs: []interface{}{&age, &email, &name, email, age},
		},
		{
			name:     "exclude null",
			where:    map[string]interface{}{"email": email},
			whereNot: map[string]interface{}{"age": age},
			wantQuery: []string{"UPDATE TableName " +
				"SET age = $1, email = $2, name = $3 " +
				"WHERE email = $4 AND age <> $5 " +
				"RETURNING age as \"age\",email as \"email\",name as \"name\""},
			wantArgs:    []interface{}{&age, &email, &name, email, age},
			excludeNull: true,
		},
	}
	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			// Act
			var query string
			var args []interface{}
			if tc.excludeNull {
				query, args = BuildUpdateWhereNotExcludeNull(tableName, src, tc.where, tc.whereNot,
					false, false)
			} else {
				query, args = BuildUpdateWhereNotIncludeNull(tableName, src, tc.where, tc.whereNot,
					false, false)
			}

			// Assert
			for _, wantQuery := range tc.wantQuery {
				assert.Contains(t, query, wantQuery)
			}
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}

func (s *SqlSuite) BuildUpdateWhereWithSetClause_WhenCalled_ShouldReturnColumnsAndValues() {
	name := "My Name"
	age := "My Age"
	email := "My Email"
	tableName := "TableName"
	retSrc := row2StructForTests{
		Name:  &name,
		Age:   &age,
		Email: &email,
	}
	setClause := squirrel.Eq{"name": name, "age": age}
	whereClause := squirrel.Eq{"email": email}

	query, args := BuildUpdateWhereWithSetClause(tableName, setClause, whereClause, retSrc)
	expectedQuery := "UPDATE TableName " +
		"SET age = $1, name = $2 " +
		"WHERE email = $3 " +
		"RETURNING age as \"age\",email as \"email\",name as \"name\""
	expectedArgs := []interface{}{&age, &email, &name, email}
	require.Equal(s.T(), expectedQuery, query)
	require.Equal(s.T(), expectedArgs, args)
}

func (s *SqlSuite) TestBuildBulkInsert() {
	tt := []interface{}{
		&row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")},
		&row2StructForTests{pointer.String("test2"), pointer.String("25"), pointer.String("email@email.com")},
	}
	query, args := BuildBulkInsert("test", tt)
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3),($4,$5,$6) RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{
		pointer.String("25"), pointer.String("email@email.com"), pointer.String("test"),
		pointer.String("25"), pointer.String("email@email.com"), pointer.String("test2"),
	}, args)
}

func (s *SqlSuite) TestBuildBulkInsertIgnore() {
	tt := []interface{}{
		&row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")},
		&row2StructForTests{pointer.String("test2"), pointer.String("25"), nil},
	}
	query, args := BuildBulkInsertIgnore("test", tt, "age")
	var nilString *string
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3),($4,$5,$6) ON CONFLICT (age) DO NOTHING RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{
		pointer.String("25"), pointer.String("email@email.com"), pointer.String("test"),
		pointer.String("25"), nilString, pointer.String("test2"),
	}, args)
}

func (s *SqlSuite) TestBuildBulkInsertReturning() {
	tt := []interface{}{
		&row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")},
		&row2StructForTests{pointer.String("test2"), pointer.String("25"), pointer.String("email@email.com")},
	}
	returnStruct := rowStructForTests{}
	query, args := BuildBulkInsertReturning("test", tt, returnStruct)
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3),($4,$5,$6) RETURNING field as \"field\",id as \"id\"", query)
	require.Equal(s.T(), []interface{}{
		pointer.String("25"), pointer.String("email@email.com"), pointer.String("test"),
		pointer.String("25"), pointer.String("email@email.com"), pointer.String("test2"),
	}, args)
}

func (s *SqlSuite) TestBuildBulkInsertWithOptionsIncludeNilFields() {
	tt := []interface{}{
		&row2StructForTests{pointer.String("test"), nil, nil},
		&row2StructForTests{pointer.String("test2"), nil, pointer.String("email@email.com")},
	}
	options := InsertOptions{IncludeNilFields: true}
	query, args := BuildBulkInsertWithOptions("test", tt, options)
	var nilString *string
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3),($4,$5,$6) RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{
		nilString, nilString, pointer.String("test"),
		nilString, pointer.String("email@email.com"), pointer.String("test2"),
	}, args)
}

func (s *SqlSuite) TestBuildBulkInsertWithOptionsIncludeIDField() {
	tt := []interface{}{
		&rowStructIntegerForTests{nil, nil},
		&rowStructIntegerForTests{nil, pointer.Int(1)},
	}
	options := InsertOptions{IncludeNilFields: true, IncludeIDField: false}
	query, args := BuildBulkInsertWithOptions("test", tt, options)
	var nilInt *int
	require.Equal(s.T(), "INSERT INTO test (field) VALUES ($1),($2) RETURNING field as \"field\",id as \"id\"", query)
	require.Equal(s.T(), []interface{}{
		nilInt, pointer.Int(1),
	}, args)
}

func (s *SqlSuite) TestBuildBulkInsertWithOptionsExcludeFields() {
	tt := []interface{}{
		&row2StructForTests{pointer.String("test"), nil, nil},
		&row2StructForTests{pointer.String("test2"), nil, pointer.String("email@email.com")},
	}
	options := InsertOptions{IncludeNilFields: true, ExcludeFields: []string{"email"}}
	query, args := BuildBulkInsertWithOptions("test", tt, options)
	var nilString *string
	require.Equal(s.T(), "INSERT INTO test (age,name) VALUES ($1,$2),($3,$4) RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{
		nilString, pointer.String("test"),
		nilString, pointer.String("test2"),
	}, args)
}

func (s *SqlSuite) TestBuildUpsertWithOptions() {
	tt := &row2StructForTests{pointer.String("test"), nil, nil}
	options := InsertOptions{IncludeNilFields: false, IncludeIDField: false}
	query, args := BuildUpsertWithOptions("test", tt, "id", options)
	require.Contains(s.T(), query, "INSERT INTO test (name) VALUES ($1) ON CONFLICT (id)")
	require.Contains(s.T(), query, "DO UPDATE SET name = EXCLUDED.name RETURNING age as \"age\",email as \"email\",name as \"name\"")
	require.Equal(s.T(), []interface{}{
		pointer.String("test"),
	}, args)
}

func (s *SqlSuite) TestBuildInsertWithSuffix() {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	query, args := BuildInsertWithSuffix("test", tt, "ON CONFLICT (email) DO NOTHING RETURNING age as \"age\",email as \"email\",name as \"name\"")
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING RETURNING age as \"age\",email as \"email\",name as \"name\"", query)
	require.Equal(s.T(), []interface{}{pointer.String("25"), pointer.String("email@email.com"), pointer.String("test")}, args)
}

func (s *SqlSuite) TestBuildInsertWithSuffix_whenSuffixIsEmpty() {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	query, args := BuildInsertWithSuffix("test", tt, "")
	require.Equal(s.T(), "INSERT INTO test (age,email,name) VALUES ($1,$2,$3) ", query)
	require.Equal(s.T(), []interface{}{pointer.String("25"), pointer.String("email@email.com"), pointer.String("test")}, args)
}

type rowStructForTests struct {
	Id    *string `db:"id"`
	Field *string `db:"field"`
}

type rowStructIntegerForTests struct {
	Id    *string `db:"id"`
	Field *int    `db:"field"`
}

type row2StructForTests struct {
	Name  *string `db:"name"`
	Age   *string `db:"age"`
	Email *string `db:"email"`
}

func contains(s []interface{}, e *string) bool {
	for _, a := range s {
		if a == e {
			return true
		} else {
			str, _ := a.(*string)
			if str != nil && e != nil {
				if *str == *e {
					return true
				}
			}
		}
	}
	return false
}

func containsString(s []interface{}, e string) bool {
	for _, a := range s {
		str, _ := a.(string)
		if str == e {
			return true
		}
	}
	return false
}

func TestSqlSuite(t *testing.T) {
	suite.Run(t, &SqlSuite{})
}

func BenchmarkMapStruct(b *testing.B) {
	// BenchmarkMapStruct-12      	 2000000	       646 ns/op	     736 B/op	       4 allocs/op
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	for n := 0; n < b.N; n++ {
		MapStruct(tt, false, false)
	}
}

func BenchmarkSliceStructSorted(b *testing.B) {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	for n := 0; n < b.N; n++ {
		SliceStructSorted(tt, false, false)
	}
}

func BenchmarkSliceStructSortedWithExcludes(b *testing.B) {
	tt := &row2StructForTests{pointer.String("test"), pointer.String("25"), pointer.String("email@email.com")}
	for n := 0; n < b.N; n++ {
		SliceStructSortedWithExcludes(tt, false, false, []string{"email"})
	}
}
