package pgtest

import (
	"database/sql"
	"errors"
	"reflect"

	"github.com/husio/feedstream/pg"
)

// DB impements mock for pg.Database interface.
//
// Whenever DB instance method is called, it's stack entry is removed and
// compared with call being made. DB fails if method notation does not match
// removed from stack entry, otherwise it returned defined by the same entry
// values.
//
type DB struct {
	Stack  []ResultMock
	Fatalf func(string, ...interface{})
}

// ResultMock defines result of DB method call. It must define Method name
// (Get, Select) that will be matched and result that DB call should return.
type ResultMock struct {
	Method string
	Result interface{}
	Err    error
}

// ExecResultMock defines result of DB.Exec call.
type ExecResultMock struct {
	InsertID int64
	Affected int64
	Err      error
}

func (m *ExecResultMock) LastInsertId() (int64, error) {
	return m.InsertID, m.Err
}

func (m *ExecResultMock) RowsAffected() (int64, error) {
	return m.Affected, m.Err
}

func (db *DB) Beginx() (pg.Connection, error) {
	mock := db.pop("Beginx", "", nil)
	if want, got := "Beginx", mock.Method; want != got {
		db.Fatalf("want %q, got %q", want, got)
		return nil, ErrUnexpectedCall
	}
	return db, mock.Err
}

func (db *DB) Commit() error {
	mock := db.pop("Commit", "", nil)
	if want, got := "Commit", mock.Method; want != got {
		db.Fatalf("want %q, got %q", want, got)
		return ErrUnexpectedCall
	}
	return mock.Err
}

func (db *DB) Rollback() error {
	mock := db.pop("Rollback", "", nil)
	if want, got := "Rollback", mock.Method; want != got {
		db.Fatalf("want %q, got %q", want, got)
		return ErrUnexpectedCall
	}
	return mock.Err
}

func (db *DB) Get(dest interface{}, query string, args ...interface{}) error {
	mock := db.pop("Get", query, args)
	if want, got := "Get", mock.Method; want != got {
		db.Fatalf("want %q, got %q call", want, got)
		return ErrUnexpectedCall
	}
	if mock.Result != nil {
		copyData(dest, mock.Result)
	}
	return mock.Err
}

func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	mock := db.pop("Select", query, args)
	if want, got := "Select", mock.Method; want != got {
		db.Fatalf("want %q, got %q call", want, got)
		return ErrUnexpectedCall
	}
	if mock.Result != nil {
		copyData(dest, mock.Result)
	}
	return mock.Err
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	mock := db.pop("Exec", query, args)
	if want, got := "Exec", mock.Method; want != got {
		db.Fatalf("want %q, got %q call", want, got)
		return nil, ErrUnexpectedCall
	}

	var (
		res sql.Result
		ok  bool
	)
	if mock.Result != nil {
		res, ok = mock.Result.(sql.Result)
		if !ok {
			db.Fatalf("'Exec' result must be sql.Result type, got %T instead", mock.Result)
		}
	}
	return res, mock.Err
}

func (db *DB) pop(method, query string, args []interface{}) *ResultMock {
	if len(db.Stack) == 0 {
		db.Fatalf("mock call stack empty: %q %s %+v", method, query, args)
		return nil
	}
	mock := db.Stack[0]
	db.Stack = db.Stack[1:]
	return &mock
}

func copyData(dst, src interface{}) {
	s := reflect.ValueOf(src)
	switch s.Kind() {
	case reflect.Ptr:
		starX := s.Elem()
		y := reflect.New(starX.Type())
		starY := y.Elem()
		starY.Set(starX)
		reflect.ValueOf(dst).Elem().Set(y.Elem())
	case reflect.Slice:
		panic("cannot copy data from slice, pass address instead")
	default:
		dst = s.Interface()
	}
}

var ErrUnexpectedCall = errors.New("unexpected call")
