package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gator/internal/database"
)

const (
	userCacheTTL    = 5 * time.Minute
	userGenKey      = "user:gen"
	userKeyTemplate = "user:gen:%d:name:%s"
)

// UserCache wraps a Cache with cache-aside reads and write-through/
// invalidation for database.User, keyed by name (the lookup MiddlewareLoggedIn
// does on every authenticated command — the hottest read path in the CLI).
//
// Invalidation uses a generation counter rather than deleting individual
// keys: every cache key embeds the current generation, and Reset bumps it
// with a single atomic INCR. Old entries are simply never looked up again
// and expire on their own via TTL — no pattern-based SCAN+DEL needed.
//
// Every method fails open: a cache error (backend down, bad data, etc.) is
// treated as a miss/no-op rather than propagated, so a cache outage can
// never break a command — it just stops speeding it up.
type UserCache struct {
	cache Cache
}

func NewUserCache(c Cache) *UserCache {
	return &UserCache{cache: c}
}

func (uc *UserCache) keyFor(ctx context.Context, name string) (string, bool) {
	gen, _, err := uc.cache.Get(ctx, userGenKey)
	if err != nil {
		return "", false
	}
	if gen == "" {
		gen = "0"
	}
	return fmt.Sprintf(userKeyTemplate, parseGenOrZero(gen), name), true
}

// Get returns the cached user for name, and whether it was found. A miss
// (not found, or any cache error) always returns (User{}, false) — callers
// should fetch from the database and call Set on the result.
func (uc *UserCache) Get(ctx context.Context, name string) (database.User, bool) {
	key, ok := uc.keyFor(ctx, name)
	if !ok {
		return database.User{}, false
	}

	val, found, err := uc.cache.Get(ctx, key)
	if err != nil || !found {
		return database.User{}, false
	}

	var u database.User
	if err := json.Unmarshal([]byte(val), &u); err != nil {
		return database.User{}, false
	}
	return u, true
}

// Set caches u, keyed by its current name. Errors are swallowed: a failed
// write just means the next lookup misses and falls back to the database.
func (uc *UserCache) Set(ctx context.Context, u database.User) {
	key, ok := uc.keyFor(ctx, u.Name)
	if !ok {
		return
	}
	data, err := json.Marshal(u)
	if err != nil {
		return
	}
	_ = uc.cache.Set(ctx, key, string(data), userCacheTTL)
}

// InvalidateAll discards every cached user by bumping the generation
// counter, so all previously-issued keys stop being looked up. Call this
// after any operation that changes user data out from under cached entries
// in a way Set can't fix up directly (currently just Reset's full wipe).
func (uc *UserCache) InvalidateAll(ctx context.Context) {
	_, _ = uc.cache.Incr(ctx, userGenKey)
}

// parseGenOrZero parses a small non-negative decimal counter value.
// Cache.Incr only ever produces digit strings, so a parse failure means
// corrupted or unexpected cache content — treated as generation 0 rather
// than erroring, since keyFor's caller already fails open on any cache
// trouble.
func parseGenOrZero(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
