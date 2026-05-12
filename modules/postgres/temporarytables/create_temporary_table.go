package temporarytables

import (
	"context"
	"fmt"
	"strings"

	postgresValidationError "compass.com/postgres/temporarytables/errors"
	"compass.com/postgres/transaction"
)

type TemporaryTablesObjectImpl struct {
}

func NewTemporartTablesObjectImpl() *TemporaryTablesObjectImpl {
	return &TemporaryTablesObjectImpl{}
}

const (
	ColumnNameIds   = "ids"
	TableNamePrefix = "temp_"
)

var (
	DefaultIdsColumnType = "uuid"
)

func (pa *TemporaryTablesObjectImpl) CreateTemporaryTableWithIds(
	ctx context.Context,
	tx transaction.TransactionContext,
	tableName string,
	ids []string,
	idsType *string,
) (
	err error,
) {
	if len(tableName) == 0 {
		return postgresValidationError.NewValidationError("ObjectCreateTemporaryTableWithIds", postgresValidationError.NewNilError("tableName"))
	}

	if err := dropTemporaryTable(ctx, tx, tableName); err != nil {
		return err
	}
	if idsType == nil {
		idsType = &DefaultIdsColumnType
	}
	var createTempTableStatement string
	numVals := len(ids)
	flatArgs := make([]interface{}, 0, numVals)
	if numVals > 0 {
		createTempTableStatement = `
			CREATE TEMP TABLE %s%s
				(%s)
			ON COMMIT DROP
			AS (
				-- Let's be really sure of the type of the columns!
				SELECT cast(a AS %s)
				FROM (VALUES %s) AS t(a));
		`
		valuesPlaceHolder := "(?)" + strings.Repeat(", (?)", numVals-1)
		createTempTableStatement = fmt.Sprintf(createTempTableStatement, TableNamePrefix, tableName, ColumnNameIds, *idsType, valuesPlaceHolder)
		createTempTableStatement = tx.Rebind(createTempTableStatement)
		for _, id := range ids {
			flatArgs = append(flatArgs, id)
		}
	} else {
		createTempTableStatement = `
			CREATE TEMP TABLE %s%s (
				%s %s
			) ON COMMIT DROP;
		`
		createTempTableStatement = fmt.Sprintf(createTempTableStatement, TableNamePrefix, tableName, ColumnNameIds, *idsType)
	}
	if _, err := tx.ExecContext(ctx, createTempTableStatement, flatArgs...); err != nil {
		return postgresValidationError.NewTemporaryTablesError(err)
	}

	return nil
}

func dropTemporaryTable(
	ctx context.Context,
	tx transaction.TransactionContext,
	tableName string,
) (
	err error,
) {
	dropTableStatement := `DROP TABLE IF EXISTS %s%s;`
	dropTableStatement = fmt.Sprintf(dropTableStatement, TableNamePrefix, tableName)
	if _, err := tx.ExecContext(ctx, dropTableStatement); err != nil {
		return postgresValidationError.NewTemporaryTablesError(err)
	}
	return nil
}
