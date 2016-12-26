package pgtest

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/husio/feedstream/pg"
)

// CreateDB connect to PostgreSQL instance, create database and return
// connection to it.
//
// Unless option is provided, defaults are used:
//   * Database name: test_database_<creation time in unix ns>
//   * Host: localhost
//   * Port: 5432
//   * SSLMode: disable
//   * User: postgres
//
// Function connects to 'postgres' database first to create new database.
func CreateDB(t *testing.T, o *DBOpts) *sql.DB {
	if o == nil {
		o = &DBOpts{}
	}
	assignDefaultOpts(o)

	connstr := fmt.Sprintf(
		"host='%s' port='%d' user='%s' dbname='postgres' sslmode='%s'",
		o.Host, o.Port, o.User, o.SSLMode)
	db, err := sql.Open("postgres", connstr)
	if err != nil {
		t.Skipf("cannot connect to postgres: %s", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("cannot ping postgres: %s", err)
	}

	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", o.DBName)); err != nil {
		t.Fatalf("cannot create database: %s", err)
		db.Close()
	}

	connstr = fmt.Sprintf(
		"host='%s' port='%d' user='%s' dbname='%s' sslmode='%s'",
		o.Host, o.Port, o.User, o.DBName, o.SSLMode)
	db, err = sql.Open("postgres", connstr)
	if err != nil {
		t.Fatalf("cannot connect to created database: %s", err)
	}

	t.Logf("test database created: %s", o.DBName)
	return db
}

// DBOpts defines options for test database connections
type DBOpts struct {
	User    string
	Port    int
	Host    string
	SSLMode string
	DBName  string
}

func assignDefaultOpts(o *DBOpts) {
	if o.DBName == "" {
		o.DBName = fmt.Sprintf("test_database_%d", time.Now().UnixNano())
	}
	if o.Host == "" {
		o.Host = "localhost"
	}
	if o.Port == 0 {
		o.Port = 5432
	}
	if o.SSLMode == "" {
		o.SSLMode = "disable"
	}
	if o.User == "" {
		o.User = "postgres"
	}
}

// CloneDB creates clone of given database.
//
// While this may speedup tests that require bootstraping with a lot of
// fixtures, be aware that content layout on the hard drive may differ from
// origin and default ordering may differ from original database.
func CloneDB(t *testing.T, from string, o *DBOpts) *sql.DB {
	if o == nil {
		o = &DBOpts{}
	}
	assignDefaultOpts(o)

	connstr := fmt.Sprintf(
		"host='%s' port='%d' user='%s' dbname='%s' sslmode='%s'",
		o.Host, o.Port, o.User, from, o.SSLMode)
	db, err := sql.Open("postgres", connstr)
	if err != nil {
		t.Skipf("cannot connect to postgres: %s", err)
	}
	defer db.Close()

	query := fmt.Sprintf("CREATE DATABASE %s WITH TEMPLATE %s", o.DBName, from)
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("cannot clone %q database: %s", from, err)
	}

	cdb, err := sql.Open("postgres", connstr)
	if err != nil {
		t.Fatalf("cannot connect to created database: %s", err)
	}

	if err := cdb.Ping(); err != nil {
		t.Fatalf("cannot ping cloned database: %s", err)
	}

	t.Logf("test database cloned: %s (from %s)", o.DBName, from)
	return cdb
}

// LoadSQL execute all SQL statements the same way as LoadSQLString function
// does, but instead of using input string, it loads statements from fixture
// file.
func LoadSQL(t *testing.T, e pg.Execer, fixture string) {
	query, err := ioutil.ReadFile(fixture)
	if err != nil {
		t.Fatalf("cannot read %q fixture: %s", fixture, err)
	}

	LoadSQLString(t, e, string(query))
}

// LoadSQLString execute all SQL statements from given string. SQL statements
// must be separated by SQLSeparator.
func LoadSQLString(t *testing.T, e pg.Execer, fixture string) {
	for _, query := range strings.Split(fixture, SQLSeparator) {
		query = strings.TrimSpace(query)
		if len(query) == 0 {
			continue
		}
		if _, err := e.Exec(query); err != nil {
			t.Fatalf("cannot load fixture: %s: %q", err, query)
		}
	}
}

// SQLSeparator defines statements separator for fixtures.
var SQLSeparator = "\n---\n"
