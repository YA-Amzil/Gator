package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

func TestUserCache_MissWhenNotSet(t *testing.T) {
	uc := NewUserCache(NewMemory())

	_, ok := uc.Get(context.Background(), "alice")
	if ok {
		t.Error("expected a miss for a user that was never cached")
	}
}

func TestUserCache_SetThenGet(t *testing.T) {
	uc := NewUserCache(NewMemory())
	ctx := context.Background()
	want := database.User{ID: uuid.New(), Name: "alice", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}

	uc.Set(ctx, want)

	got, ok := uc.Get(ctx, "alice")
	if !ok {
		t.Fatal("expected a hit after Set")
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
}

func TestUserCache_SetOverwritesPreviousValue(t *testing.T) {
	uc := NewUserCache(NewMemory())
	ctx := context.Background()

	first := database.User{ID: uuid.New(), Name: "alice"}
	uc.Set(ctx, first)

	second := database.User{ID: uuid.New(), Name: "alice"}
	uc.Set(ctx, second)

	got, ok := uc.Get(ctx, "alice")
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.ID != second.ID {
		t.Errorf("Get returned ID %v, want the most recently Set ID %v", got.ID, second.ID)
	}
}

func TestUserCache_InvalidateAll_MakesPreviousEntriesUnreachable(t *testing.T) {
	uc := NewUserCache(NewMemory())
	ctx := context.Background()

	uc.Set(ctx, database.User{ID: uuid.New(), Name: "alice"})
	if _, ok := uc.Get(ctx, "alice"); !ok {
		t.Fatal("expected a hit before invalidation")
	}

	uc.InvalidateAll(ctx)

	if _, ok := uc.Get(ctx, "alice"); ok {
		t.Error("expected a miss after InvalidateAll bumped the generation")
	}
}

func TestUserCache_SetAfterInvalidate_IsReachableAgain(t *testing.T) {
	uc := NewUserCache(NewMemory())
	ctx := context.Background()

	uc.Set(ctx, database.User{ID: uuid.New(), Name: "alice"})
	uc.InvalidateAll(ctx)

	fresh := database.User{ID: uuid.New(), Name: "alice"}
	uc.Set(ctx, fresh)

	got, ok := uc.Get(ctx, "alice")
	if !ok {
		t.Fatal("expected a hit for the entry written after invalidation")
	}
	if got.ID != fresh.ID {
		t.Errorf("Get returned ID %v, want the post-invalidation ID %v", got.ID, fresh.ID)
	}
}

// fakeErrorCache always errors, simulating a Redis outage — every UserCache
// method must fail open rather than propagate.
type fakeErrorCache struct{}

func (fakeErrorCache) Get(ctx context.Context, key string) (string, bool, error) {
	return "", false, errBoom
}
func (fakeErrorCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return errBoom
}
func (fakeErrorCache) Delete(ctx context.Context, keys ...string) error { return errBoom }
func (fakeErrorCache) Incr(ctx context.Context, key string) (int64, error) {
	return 0, errBoom
}

var errBoom = errors.New("boom")

func TestUserCache_FailsOpenOnCacheErrors(t *testing.T) {
	uc := NewUserCache(fakeErrorCache{})
	ctx := context.Background()

	// None of these should panic or error out; Get should simply miss.
	if _, ok := uc.Get(ctx, "alice"); ok {
		t.Error("expected a miss when the cache backend errors")
	}
	uc.Set(ctx, database.User{ID: uuid.New(), Name: "alice"})
	uc.InvalidateAll(ctx)
}
