package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultDialTimeout bounds how long connecting to Redis may take. The
// go-redis default (5s) plus its default 3 retries means an unreachable
// Redis would otherwise stall every single command for ~15s before falling
// back to the database — unacceptable for a CLI tool whose whole point is
// to be fast. A short timeout with no retries makes the fail-open path
// fail fast instead.
const defaultDialTimeout = 300 * time.Millisecond

func init() {
	// go-redis logs every failed connection attempt to stderr by default.
	// Since a missing/unreachable Redis is an expected, silently-handled
	// case here (UserCache falls back to the database), that noise would
	// look like a real error to users running a command with no cache
	// configured incorrectly. discardLogger suppresses it; RedisCache's own
	// errors still propagate normally to callers.
	redis.SetLogger(discardLogger{})
}

type discardLogger struct{}

func (discardLogger) Printf(ctx context.Context, format string, v ...interface{}) {}

// RedisCache is the production Cache backend. The client connects lazily
// (the redis-go driver dials on first command), so constructing a RedisCache
// never fails on its own — only a malformed URL does. A genuinely
// unreachable Redis surfaces as an error from Get/Set/Delete/Incr, which
// UserCache treats as a cache miss and falls back to the database.
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(url string) (*RedisCache, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}
	if opt.DialTimeout == 0 {
		opt.DialTimeout = defaultDialTimeout
	}
	opt.MaxRetries = -1 // fail fast instead of retrying a down Redis
	return &RedisCache{client: redis.NewClient(opt)}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("redis get %q: %w", key, err)
	}
	return val, true, nil
}

func (r *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del %v: %w", keys, err)
	}
	return nil
}

func (r *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis incr %q: %w", key, err)
	}
	return n, nil
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}
