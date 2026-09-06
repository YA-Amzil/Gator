// Package cache provides an optional caching layer, backed by Redis in
// production, sitting in front of read-heavy, write-rare database queries.
// Caching is never a hard dependency: every Cache implementation is safe to
// use when the backing store is unavailable, and callers built on top of it
// (see UserCache) always fail open to the database on any cache error.
package cache

import (
	"context"
	"time"
)

// Cache is a minimal key-value store with TTL expiry and an atomic counter,
// enough to build cache-aside read paths with generation-based invalidation
// (see UserCache) without needing pattern-based key deletion.
type Cache interface {
	// Get returns the value for key and true if present, or ("", false, nil)
	// on a miss. A non-nil error indicates the cache itself is unavailable;
	// callers should treat that the same as a miss and fall back to the
	// source of truth.
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	// Incr atomically increments key (starting from 0 if unset) and returns
	// the new value.
	Incr(ctx context.Context, key string) (int64, error)
}

// NoopCache always misses and discards writes. It's used when no cache
// backend is configured, so callers never need to nil-check or branch on
// whether caching is enabled.
type NoopCache struct{}

func NewNoop() NoopCache { return NoopCache{} }

func (NoopCache) Get(ctx context.Context, key string) (string, bool, error) { return "", false, nil }

func (NoopCache) Set(ctx context.Context, key, value string, ttl time.Duration) error { return nil }

func (NoopCache) Delete(ctx context.Context, keys ...string) error { return nil }

func (NoopCache) Incr(ctx context.Context, key string) (int64, error) { return 0, nil }
