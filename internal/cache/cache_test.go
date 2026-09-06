package cache

import (
	"context"
	"testing"
	"time"
)

func TestNoopCache_AlwaysMisses(t *testing.T) {
	c := NewNoop()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	_, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("NoopCache.Get should always miss, even after Set")
	}

	n, err := c.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("Incr returned error: %v", err)
	}
	if n != 0 {
		t.Errorf("Incr = %d, want 0", n)
	}

	if err := c.Delete(ctx, "k"); err != nil {
		t.Errorf("Delete returned error: %v", err)
	}
}

func TestMemoryCache_SetThenGet(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok || got != "v" {
		t.Errorf("Get = (%q, %v), want (\"v\", true)", got, ok)
	}
}

func TestMemoryCache_MissForUnknownKey(t *testing.T) {
	c := NewMemory()

	_, ok, err := c.Get(context.Background(), "nope")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected a miss for an unset key")
	}
}

func TestMemoryCache_ExpiresAfterTTL(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", 20*time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	_, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected the entry to have expired")
	}
}

func TestMemoryCache_ZeroTTLNeverExpires(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	_, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Error("a zero TTL should mean no expiry")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Error("expected a miss after Delete")
	}
}

func TestMemoryCache_Incr(t *testing.T) {
	c := NewMemory()
	ctx := context.Background()

	for want := int64(1); want <= 3; want++ {
		got, err := c.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr returned error: %v", err)
		}
		if got != want {
			t.Errorf("Incr = %d, want %d", got, want)
		}
	}
}
