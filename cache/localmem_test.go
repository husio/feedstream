package cache

import (
	"context"
	"testing"
)

func TestLocalMemCache(t *testing.T) {
	testCacheService(t, context.Background(), NewLocalMemCache())
}
