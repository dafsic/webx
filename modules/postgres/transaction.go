package postgres

import "github.com/jmoiron/sqlx"

// Transaction is an helper for managing the lifespan of a sqlx transaction.
// Use as:
//
//	func Foo(...) (..., err error) {
//	  ...
//	  t := postgres.NewTransaction(tx)
//	  defer t.Close(&err)
//	  // actually do stuff with t.Session(). Don't need to worry about closing or rolling back
//	  // tx, t.Close() will take care of it.
type Transaction interface {
	// Session returns the underlying transaction.
	Session() *sqlx.Tx
	// Close should be deferred. It closes the underlying sqlx.Tx if err is nil
	// If err is not nil, or if there is a panic, Close will rollback the transaction.
	Close(err *error)
}

// NewTransaction creates a new Transaction.
func NewTransaction(tx *sqlx.Tx) Transaction {
	return &baseTransaction{tx}
}

type baseTransaction struct {
	tx *sqlx.Tx
}

func (bt *baseTransaction) Session() *sqlx.Tx {
	return bt.tx
}

func (bt *baseTransaction) Close(err *error) {
	if e := recover(); e != nil || *err != nil {
		if rollbackErr := bt.tx.Rollback(); rollbackErr != nil {
			log.Errorf("Error rolling back transaction: %s", rollbackErr)
		}
		if e != nil {
			panic(e)
		}
	} else {
		*err = bt.tx.Commit()
	}
}
