package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

// setupTestRedis connects to the Redis instance named by GATOR_REDIS_URL and
// flushes its keyspace for a clean slate. Tests are skipped (not failed)
// when no Redis is reachable, so `go test ./...` still passes without it
// running — the same convention internal/database uses for Postgres.
func setupTestRedis(t *testing.T) *RedisCache {
	t.Helper()

	redisURL := os.Getenv("GATOR_REDIS_URL")
	if redisURL == "" {
		t.Skip("GATOR_REDIS_URL not set; skipping redis integration test")
	}

	rc, err := NewRedisCache(redisURL)
	if err != nil {
		t.Fatalf("NewRedisCache returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := rc.Get(ctx, "connectivity-check"); err != nil {
		rc.Close()
		t.Skipf("redis not reachable at %s: %v", redisURL, err)
	}

	t.Cleanup(func() {
		_ = rc.client.FlushDB(context.Background()).Err()
		rc.Close()
	})
	_ = rc.client.FlushDB(ctx).Err()

	return rc
}

func TestRedisCache_SetThenGet(t *testing.T) {
	rc := setupTestRedis(t)
	ctx := context.Background()

	if err := rc.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok || got != "v" {
		t.Errorf("Get = (%q, %v), want (\"v\", true)", got, ok)
	}
}

func TestRedisCache_MissForUnknownKey(t *testing.T) {
	rc := setupTestRedis(t)

	_, ok, err := rc.Get(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected a miss for an unset key")
	}
}

func TestRedisCache_ExpiresAfterTTL(t *testing.T) {
	rc := setupTestRedis(t)
	ctx := context.Background()

	if err := rc.Set(ctx, "k", "v", 50*time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	_, ok, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected the entry to have expired")
	}
}

func TestRedisCache_Delete(t *testing.T) {
	rc := setupTestRedis(t)
	ctx := context.Background()

	if err := rc.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := rc.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, ok, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected a miss after Delete")
	}
}

func TestRedisCache_Incr(t *testing.T) {
	rc := setupTestRedis(t)
	ctx := context.Background()

	for want := int64(1); want <= 3; want++ {
		got, err := rc.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr returned error: %v", err)
		}
		if got != want {
			t.Errorf("Incr = %d, want %d", got, want)
		}
	}
}

func TestUserCache_WithRealRedis(t *testing.T) {
	rc := setupTestRedis(t)
	uc := NewUserCache(rc)
	ctx := context.Background()

	// End-to-end sanity check against real Redis: the in-memory-backed
	// UserCache tests already cover the generation/invalidation logic in
	// detail, so this just confirms the RedisCache plumbing (JSON
	// marshaling through Get/Set, the generation counter via Incr) actually
	// works against a real server, not just the in-memory fake.
	if _, ok := uc.Get(ctx, "alice"); ok {
		t.Fatal("expected a miss before Set")
	}

	want := database.User{ID: uuid.New(), Name: "alice"}
	uc.Set(ctx, want)

	got, ok := uc.Get(ctx, "alice")
	if !ok {
		t.Fatal("expected a hit after Set")
	}
	if got.ID != want.ID {
		t.Errorf("Get returned ID %v, want %v", got.ID, want.ID)
	}

	uc.InvalidateAll(ctx)
	if _, ok := uc.Get(ctx, "alice"); ok {
		t.Error("expected a miss after InvalidateAll")
	}
}
