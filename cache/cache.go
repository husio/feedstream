package cache

import (
	"context"
	"errors"
	"time"
)

// CacheService represents high level cache client with custom serialization
// for stored data. It is important is that it represents cache. Provided
// implementation might flush it's memory at any time and user must be prepared
// to loose all stored data.
type CacheService interface {
	// Get value stored under given key. Returns ErrMiss if key is not
	// used.
	Get(ctx context.Context, key string, dest interface{}) error

	// Set value under given key. If key is already in use, overwrite it's
	// value with given one and set new expiration time.
	Set(ctx context.Context, key string, value interface{}, exp time.Duration) error

	// Add set value under given key only if key is not used. It returns
	// ErrConflict if trying to set value for key that is already in use.
	Add(ctx context.Context, key string, value interface{}, exp time.Duration) error

	// Del deletes value under given key. It returns ErrCacheMiss if given
	// key is not used.
	Del(ctx context.Context, key string) error
}

// BareCacheService represents cache client. It is important is that it
// represents cache. Provided implementation might flush it's memory at any
// time and user must be prepared to loose all stored data.
type BareCacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, exp time.Duration) error
	Del(ctx context.Context, key string) error
	IncrExisting(ctx context.Context, key string, delta int64) (uint64, error)
	Add(ctx context.Context, key string, value []byte, exp time.Duration) error
}

var (
	// ErrMiss is returned when performing operation on key is not in use.
	ErrMiss = errors.New("cache miss")

	// ErrConflict is returned when performing operation on existing key,
	// which cause conflict.
	ErrConflict = errors.New("conflict")
)
