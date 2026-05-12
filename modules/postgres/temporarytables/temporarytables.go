package temporarytables

import (
	"context"

	"compass.com/postgres/transaction"
)

//go:generate compassmock TemporaryTables
type TemporaryTables interface {

	// CreateTemporaryTableWithIds creates a temporary table in the database
	// of the provided transaction and inserts the provided ids in the `ids` column.
	// The ids are expected to be a set of SubjectIds or ResourceIds corresponding
	// to the results of a PermissionsQuery (ex. Permissions.ReadComponentIds).
	// As the table is dropped prior to creation, the tableName is prefixed
	// with "temp_" as a precaution.
	//
	// This temporary table is expected to be used in subsequent database queries to:
	// -- avoid passing ids around the api and db methods
	// -- limit the scope of database operations to the ids
	// -- allow the database to perform as much work as possible
	// -- return the minimum necessary data from the database
	// -- maximize simplicity and efficiency
	//
	// For example, consider the use-case of a user with Privileges.reader for 10,000 resources,
	// requesting an arbitrary filter, sort, and pagination resulting in a page of 25 resources.
	// This use-case can be facilitated using a single query where the resources table
	// is joined to the temporary table by id.
	//
	// Currently supports a maximum of 10,000 ids. We can increase this if requested (#identity_team).
	// Cannot be run on read replicas but that is not an expected use case for user transactions.
	//
	// A future version of the SDK will provide an alternative using Materialized Views into your
	// Postgres database or into a RocksDb.
	CreateTemporaryTableWithIds(
		ctx context.Context,
		tx transaction.TransactionContext,
		tableName string,
		ids []string,
		idsType *string,
	) (
		err error,
	)
}
