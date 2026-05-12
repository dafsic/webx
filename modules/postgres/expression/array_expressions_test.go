package expression_test

import (
	"testing"

	"github.com/lib/pq"

	"compass.com/postgres/expression"
	"github.com/stretchr/testify/suite"
)

type ExpressionSuite struct {
	suite.Suite
}

func TestExpressions(t *testing.T) {
	suite.Run(t, &ExpressionSuite{})
}

func (e *ExpressionSuite) TestSerializeOverlapOfKeyAndArrayValueExpression() {
	expr := expression.ArrayOverlap{"alpha": []int{1, 2, 3}}

	sql, args, err := expr.ToSql()

	e.Equal("alpha && ?", sql)
	e.Equal([]interface{}{
		pq.Array([]int{1, 2, 3}),
	}, args)
	e.Nil(err)
}

func (e *ExpressionSuite) TestSerializeKeyContainingArrayValueExpression() {
	expr := expression.ArrayContains{"alpha": []int{1, 2, 3}}

	sql, args, err := expr.ToSql()

	e.Equal("alpha @> ?", sql)
	e.Equal([]interface{}{
		pq.Array([]int{1, 2, 3}),
	}, args)
	e.Nil(err)
}

func (e *ExpressionSuite) TestSerializeKeyContainedByArrayValueExpression() {
	expr := expression.ArrayContainedBy{"alpha": []int{1, 2, 3}}

	sql, args, err := expr.ToSql()

	e.Equal("alpha <@ ?", sql)
	e.Equal([]interface{}{
		pq.Array([]int{1, 2, 3}),
	}, args)
	e.Nil(err)
}

func (e *ExpressionSuite) TestSerializeMultipleKeysAsAndExpression() {
	expr := expression.ArrayOverlap{
		"alpha": []int{1},
		"bravo": []int{2, 2},
	}

	sql, args, err := expr.ToSql()

	e.Equal("alpha && ? AND bravo && ?", sql)
	e.Equal([]interface{}{
		pq.Array([]int{1}),
		pq.Array([]int{2, 2}),
	}, args)
	e.Nil(err)
}

func (e *ExpressionSuite) TestFailToSerializeNonListExpression() {
	expr := expression.ArrayOverlap{
		"alpha": "not an array value",
	}

	_, _, err := expr.ToSql()

	e.EqualError(err, "must use array or slice with array operators")
}
