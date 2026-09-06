package cache

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	value     string
	expiresAt time.Time // zero means no expiry
}

// MemoryCache is an in-process Cache implementation. It exists mainly so
// tests can exercise real cache-aside and invalidation logic (hits, misses,
// TTL expiry, generation bumps) fast and deterministically, without a real
// Redis instance — NewRedisCache is the production backend.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
}

func NewMemory() *MemoryCache {
	return &MemoryCache{entries: make(map[string]memoryEntry)}
}

func (m *MemoryCache) Get(ctx context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[key]
	if !ok {
		return "", false, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(m.entries, key)
		return "", false, nil
	}
	return entry.value, true, nil
}

func (m *MemoryCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	m.entries[key] = memoryEntry{value: value, expiresAt: expiresAt}
	return nil
}

func (m *MemoryCache) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.entries, key)
	}
	return nil
}

func (m *MemoryCache) Incr(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var n int64
	if entry, ok := m.entries[key]; ok && (entry.expiresAt.IsZero() || time.Now().Before(entry.expiresAt)) {
		// Best-effort parse; a corrupt counter value just resets to 1.
		for _, c := range entry.value {
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int64(c-'0')
		}
	}
	n++
	m.entries[key] = memoryEntry{value: itoa(n)}
	return n, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
