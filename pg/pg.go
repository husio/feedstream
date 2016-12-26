package pg

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Getter is generic interface for getting single entity
type Getter interface {
	Get(dest interface{}, query string, args ...interface{}) error
}

// Selector is generic interface for getting multiple enties
type Selector interface {
	Select(dest interface{}, query string, args ...interface{}) error
}

// Execer is generic interface for executing SQL query with no result
type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type Database interface {
	Beginx() (Connection, error)
	Getter
	Selector
	Execer
	Close() error
}

type Connection interface {
	Getter
	Selector
	Execer
	Rollback() error
	Commit() error
}

// sqlxDb wraps sqlx.DB structure and provides custom function notations that
// can be easily mocked. This wrapper is required, because of sqlx.DB's Beginx
// method notation
type sqlxDb struct {
	dbx *sqlx.DB
}

var _ Database = (*sqlxDb)(nil)

func Use(db *sql.DB) Database {
	dbx := sqlx.NewDb(db, "postgres")
	return &sqlxDb{dbx: dbx}
}

func Connect(credentials string) (Database, error) {
	dbx, err := sqlx.Connect("postgres", credentials)
	if err != nil {
		return nil, err
	}
	return &sqlxDb{dbx: dbx}, nil
}

func (x *sqlxDb) Beginx() (Connection, error) {
	tx, err := x.dbx.Beginx()
	return &sqlxTx{tx: tx}, castErr(err)
}

func (x *sqlxDb) Get(dest interface{}, query string, args ...interface{}) error {
	err := x.dbx.Get(dest, query, args...)
	return castErr(err)
}

func (x *sqlxDb) Select(dest interface{}, query string, args ...interface{}) error {
	err := x.dbx.Select(dest, query, args...)
	return castErr(err)
}

func (x *sqlxDb) Exec(query string, args ...interface{}) (sql.Result, error) {
	res, err := x.dbx.Exec(query, args...)
	return res, castErr(err)
}

func (x *sqlxDb) Close() error {
	err := x.dbx.Close()
	return castErr(err)
}

type sqlxTx struct {
	tx *sqlx.Tx
}

var _ Connection = (*sqlxTx)(nil)

func (x *sqlxTx) Get(dest interface{}, query string, args ...interface{}) error {
	err := x.tx.Get(dest, query, args...)
	return castErr(err)
}

func (x *sqlxTx) Select(dest interface{}, query string, args ...interface{}) error {
	err := x.tx.Select(dest, query, args...)
	return castErr(err)
}

func (x *sqlxTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	res, err := x.tx.Exec(query, args...)
	return res, castErr(err)
}

func (x *sqlxTx) Rollback() error {
	err := x.tx.Rollback()
	return castErr(err)
}

func (x *sqlxTx) Commit() error {
	err := x.tx.Commit()
	return castErr(err)
}

// castErr inspect given error and replace generic SQL error with easier to
// compare equivalent.
//
// See http://www.postgresql.org/docs/current/static/errcodes-appendix.html
func castErr(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err, ok := err.(*pq.Error); ok && err.Code == "23505" {
		return ErrConflict
	}
	return err
}

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)
