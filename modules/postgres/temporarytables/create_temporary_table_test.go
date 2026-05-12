package temporarytables_test

import (
	"context"
	"fmt"
	"testing"

	mocktransaction "compass.com/mocks/postgres/transaction"
	tempImpl "compass.com/postgres/temporarytables"
	postgresValidationErrors "compass.com/postgres/temporarytables/errors"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	resourceIdsForTest = []string{"1", "2", "3", "4", "5"}
	tableNameForTest   = "table_name"
)

type TemporaryTableSuite struct {
	suite.Suite
}

func TestTemporaryTables(t *testing.T) {
	suite.Run(t, &TemporaryTableSuite{})
}

type SubjectAndMocks struct {
	subject       *tempImpl.TemporaryTablesObjectImpl
	context       context.Context
	txContextMock *mocktransaction.TransactionContext
}

func createSubjectAndMocks() *SubjectAndMocks {
	txContextMock := new(mocktransaction.TransactionContext)
	subject := tempImpl.NewTemporartTablesObjectImpl()
	return &SubjectAndMocks{
		subject:       subject,
		context:       context.Background(),
		txContextMock: txContextMock,
	}
}

func (s *TemporaryTableSuite) TestCreateTemporaryTable() {
	sam := createSubjectAndMocks()
	requestContext := context.Background()
	isExpectedQueryStr := "\n\t\t\tCREATE TEMP TABLE temp_table_name\n\t\t\t\t(ids)\n\t\t\tON COMMIT DROP\n\t\t\tAS (\n\t\t\t\t-- Let's be really sure of the type of the columns!\n\t\t\t\tSELECT cast(a AS uuid)\n\t\t\t\tFROM (VALUES (?), (?), (?), (?), (?)) AS t(a));\n\t\t"
	sam.txContextMock.
		On("ExecContext", requestContext, isExpectedQueryStr, "1", "2", "3", "4", "5").
		Return(nil, nil)
	isExpectedDropTableQueryStr := fmt.Sprintf("DROP TABLE IF EXISTS temp_%s;", tableNameForTest)
	sam.txContextMock.
		On("ExecContext", requestContext, isExpectedDropTableQueryStr, mock.Anything).
		Return(nil, nil)
	sam.txContextMock.
		On("Rebind", mock.Anything).
		Return(isExpectedQueryStr)

	err := sam.subject.CreateTemporaryTableWithIds(requestContext, sam.txContextMock, tableNameForTest, resourceIdsForTest, nil)
	s.Nil(err)
}

func (s *TemporaryTableSuite) TestCreateTemporaryTableValidParameters() {
	requestContext := context.Background()
	var emptySliceOfIds []string
	sam := createSubjectAndMocks()
	isExpectedQueryStr := "\n\t\t\tCREATE TEMP TABLE temp_table_name (\n\t\t\t\tids uuid\n\t\t\t) ON COMMIT DROP;\n\t\t"
	sam.txContextMock.
		On("ExecContext", requestContext, isExpectedQueryStr).
		Return(nil, nil)
	isExpectedDropTableQueryStr := fmt.Sprintf("DROP TABLE IF EXISTS temp_%s;", tableNameForTest)
	sam.txContextMock.
		On("ExecContext", requestContext, isExpectedDropTableQueryStr).
		Return(nil, nil)
	err := sam.subject.CreateTemporaryTableWithIds(requestContext, sam.txContextMock, tableNameForTest, emptySliceOfIds, nil)
	s.Nil(err)
}

func (s *TemporaryTableSuite) TestCreateTemporaryTableInValidParameters() {
	requestContext := context.Background()
	sam := createSubjectAndMocks()
	tableName := ""
	sam.txContextMock.
		On("ExecContext", requestContext, mock.Anything, mock.Anything).
		Return(nil, nil)
	err := sam.subject.CreateTemporaryTableWithIds(requestContext, sam.txContextMock, tableName, resourceIdsForTest, nil)
	s.NotNil(err)
	_, isValidationError := errors.Cause(err).(*postgresValidationErrors.ValidationError)
	s.Equal(isValidationError, true)
}

func (s *TemporaryTableSuite) TestDatabaseRequestFails() {
	sam := createSubjectAndMocks()
	requestContext := context.Background()
	execContextError := errors.New("problem")
	sam.txContextMock.
		On("ExecContext", requestContext, mock.Anything, mock.Anything).
		Return(nil, execContextError)
	err := sam.subject.CreateTemporaryTableWithIds(requestContext, sam.txContextMock, tableNameForTest, resourceIdsForTest, nil)
	s.NotNil(err)
}
