package auth

import (
	"context"
	"reflect"
	"testing"

	"github.com/husio/feedstream/pg"
	"github.com/husio/feedstream/pg/pgtest"
)

func TestAccountsDatabaseEnsureExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := pg.Use(pgtest.CreateDB(t, nil))
	defer db.Close()

	pgtest.LoadSQLString(t, db, Schema)

	a := accountsdb{db: db}

	u, err := a.EnsureExists(ctx, User{
		Provider:   "x",
		Name:       "JohnSmith",
		ProfileURL: "https://example.com/johnsmith",
	})
	if err != nil {
		t.Fatalf("cannot create user: %s", err)
	}
	if u.AccountID == 0 {
		t.Fatalf("no account id assigned: %+v", u)
	}

	// user must match by profider/profile
	u2, err := a.EnsureExists(ctx, User{
		Provider:   "x",
		ProfileURL: "https://example.com/johnsmith",
	})
	if err != nil {
		t.Fatalf("cannot ensure user matching by profile/provider: %s", err)
	}
	if !reflect.DeepEqual(u, u2) {
		t.Fatalf("user difference: \n%#v\n%#v", u, u2)
	}

	// user must match by id
	u3, err := a.EnsureExists(ctx, User{
		AccountID: u.AccountID,
	})
	if err != nil {
		t.Fatalf("cannot ensure user matching by account id: %s", err)
	}
	if !reflect.DeepEqual(u, u3) {
		t.Fatalf("user difference: \n%#v\n%#v", u, u3)
	}

	// radom user
	_, err = a.EnsureExists(ctx, User{
		Provider:   "x",
		ProfileURL: "https://example.com/randomuser",
	})
	if err != nil {
		t.Fatalf("cannot create random user: %s", err)
	}

	var cnt int
	if err := db.Get(&cnt, `SELECT COUNT(*) FROM accounts`); err != nil {
		t.Fatalf("cannot count accounts: %s", err)
	}
	if cnt != 2 {
		t.Fatalf("want two accounts, got %d", cnt)
	}
}
