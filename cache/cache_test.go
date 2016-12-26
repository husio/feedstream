package cache

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// testCacheService run standard test set on given cache service
// implementation. All implementations must pass this in order to be
// compatible.
func testCacheService(t *testing.T, ctx context.Context, c CacheService) {
	cname := fmt.Sprintf("%T", c)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// ghosts don't exists, make sure this implementation knows this
	if err := c.Get(ctx, "ghost", nil); err != ErrMiss {
		t.Fatalf("%s get: want ErrMiss, got %q", cname, err)
	}
	if err := c.Del(ctx, "ghost"); err != ErrMiss {
		t.Fatalf("%s delete: want ErrMiss, got %q", cname, err)
	}

	type person struct {
		Name string
		Age  int
	}

	bob := person{
		Name: "bob",
		Age:  52,
	}
	// simple set must always work
	for i := 0; i < 3; i++ {
		if err := c.Set(ctx, "bob", &bob, time.Minute); err != nil {
			t.Fatalf("%s set: cannot set bob: %s", cname, err)
		}

		// get previously stored bob - result must be equal to initial structure
		var bob2 person
		if err := c.Get(ctx, "bob", &bob2); err != nil {
			t.Fatalf("%s get: cannot get bob: %s", cname, err)
		}
		if !reflect.DeepEqual(bob, bob2) {
			t.Fatalf("%s: bob compare: want %+v, got %+v", cname, bob, bob2)
		}
	}

	// add must fail when key is already used
	if err := c.Add(ctx, "bob", &bob, time.Minute); err != ErrConflict {
		t.Fatalf("%s add: want ErrConflict, got %+v", cname, err)
	}

	// make sure deleting works as expected
	if err := c.Del(ctx, "bob"); err != nil {
		t.Fatalf("%s delete: cannot remove: %q", cname, err)
	}

	// now that bob was deleted, we must be able to add it again
	if err := c.Add(ctx, "bob", &bob, time.Second); err != nil {
		t.Fatalf("%s add: cannot add: %+v", cname, err)
	}
	var bob3 person
	if err := c.Get(ctx, "bob", &bob3); err != nil {
		t.Fatalf("%s get: cannot get bob: %+v", cname, err)
	}
	if !reflect.DeepEqual(bob, bob3) {
		t.Fatalf("%s bob compare: want %+v, got %+v", cname, bob, bob3)
	}

	// make sure the cache expires
	// memcache does not tolerate duration more granular than 1s
	time.Sleep(time.Second)

	// expiration must work
	if err := c.Get(ctx, "bob", &bob3); err != ErrMiss {
		t.Fatalf("%s bob was supposed to expire, instead got %+v, %+v", cname, err, bob3)
	}
}
