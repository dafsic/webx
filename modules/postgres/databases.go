package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	ddsql "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

// Database is the interface for interacting with postgres databases using sqlx.
//
//go:generate compassmock Database
type Database interface {
	IsEnabled() bool
	Session() *sqlx.DB
	Config() *DbConf
	Transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error
	TransactReadOnly(ctx context.Context, txFunc func(tx *sqlx.Tx) error) error
}

type DatabaseImpl struct {
	db     *sqlx.DB
	config *DbConf
}

func NewDatabaseImpl(db *sqlx.DB, config *DbConf) *DatabaseImpl {
	return &DatabaseImpl{db: db, config: config}
}

func (d *DatabaseImpl) IsEnabled() bool {
	return d.db != nil
}

func (d *DatabaseImpl) Session() *sqlx.DB {
	return d.db
}

func (d *DatabaseImpl) Config() *DbConf {
	return d.config
}

func (db *DatabaseImpl) Transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error) (err error) {
	return db.transact(ctx, txFunc, nil)
}

func (db *DatabaseImpl) TransactReadOnly(ctx context.Context, txFunc func(tx *sqlx.Tx) error) (err error) {
	return db.transact(ctx, txFunc, &sql.TxOptions{ReadOnly: true})
}

func (db *DatabaseImpl) transact(ctx context.Context, txFunc func(tx *sqlx.Tx) error, options *sql.TxOptions) (err error) {
	tx, err := db.Session().BeginTxx(ctx, options)
	if err != nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				log.Errorf("error during rollback: %s", rollbackErr)
			}
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			rollbackErr := tx.Rollback() // err is non-nil; don't change it
			if rollbackErr != nil {
				err = errors.Wrap(err, fmt.Sprintf("error during rollback: %s", rollbackErr))
			}
		} else {
			err = tx.Commit() // err is nil; if Commit returns error update err
		}
	}()
	err = txFunc(tx)
	return
}

func dialDatabase(name string, conf *DbConf, ddEnabled bool) (Database, error) {
	if !conf.Enabled {
		log.Warningf("Postgres database %s is disabled.", name)
		return &DatabaseImpl{db: nil, config: nil}, nil
	}
	sanitize := func(s string) string {
		return strings.Replace(s, "'", "\\'", -1)
	}
	connStrNoPassword := fmt.Sprintf("host='%s' port=%d user='%s' password='%s' dbname='%s' sslmode='%s' search_path='%s'",
		sanitize(conf.Hostname),
		conf.Port,
		sanitize(conf.Username),
		"%s",
		sanitize(conf.DbName),
		sanitize(conf.SslMode),
		sanitize(conf.DefaultSearchPath))
	if conf.StatementTimeout != 0 {
		connStrNoPassword += fmt.Sprintf(" statement_timeout=%d", conf.StatementTimeout)
	}
	log.Debugf("Connecting to "+connStrNoPassword+" (max_idle=%d, max_open=%d, max_lifetime=%v)",
		"****",
		conf.MaxIdleConns,
		conf.MaxOpenConns,
		conf.MaxConnLifetime)
	connStr := fmt.Sprintf(connStrNoPassword, sanitize(conf.Password))

	driver := "postgres"
	if conf.Trace {
		driver = instrumentedPgDriverName
	}
	var sqlDb *sql.DB
	var err error
	if ddEnabled {
		sqlDb, err = ddsql.Open(driver, connStr)
	} else {
		sqlDb, err = sql.Open(driver, connStr)
	}
	if err != nil {
		return nil, err
	}
	sqlDb.SetMaxIdleConns(conf.MaxIdleConns)
	sqlDb.SetMaxOpenConns(conf.MaxOpenConns)
	sqlDb.SetConnMaxLifetime(conf.MaxConnLifetime)
	// HACK(ugo): Use postgres as driverName with sqlx until
	// https://github.com/jmoiron/sqlx/pull/401/ is available.
	db := sqlx.NewDb(sqlDb, "postgres")
	log.Infof("Connected to %s", name)
	return &DatabaseImpl{db: db, config: conf}, nil
}

// Databases is a collection of named Database objects.
// Typically you will want to access get your Databases as an injectable
// provided by the postgres.Module
type Databases struct {
	items map[string]Database
}

// All database names that have been configured in the postgres.Module.
// Includes databases that are NOT flag-enabled.
func (p *Databases) Names() []string {
	names := make([]string, len(p.items))
	i := 0
	for n := range p.items {
		names[i] = n
		i++
	}
	return names
}

// Get the sql database for a given name. nil if this database is disabled.
func (p *Databases) Get(name string) Database {
	return p.items[name]
}
