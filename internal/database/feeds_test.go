package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateAndGetFeedByURL(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	created, err := q.CreateFeed(ctx, CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      "Hacker News",
		Url:       "https://news.ycombinator.com/rss",
		UserID:    user.ID,
	})
	if err != nil {
		t.Fatalf("CreateFeed returned error: %v", err)
	}
	if created.LastFetchedAt.Valid {
		t.Error("LastFetchedAt should be NULL for a newly created feed")
	}
	if created.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 for a newly created feed", created.ConsecutiveFailures)
	}
	if created.LastFetchError.Valid {
		t.Error("LastFetchError should be NULL for a newly created feed")
	}

	found, err := q.GetFeedByURL(ctx, "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("GetFeedByURL ID = %v, want %v", found.ID, created.ID)
	}
}

func TestCreateFeed_DuplicateURLFails(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	params := CreateFeedParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "HN", Url: "https://news.ycombinator.com/rss", UserID: user.ID}
	if _, err := q.CreateFeed(ctx, params); err != nil {
		t.Fatalf("first CreateFeed returned error: %v", err)
	}

	params.ID = uuid.New()
	params.Name = "HN Again"
	if _, err := q.CreateFeed(ctx, params); err == nil {
		t.Fatal("expected an error creating a feed with a duplicate URL, got nil")
	}
}

func TestGetFeeds_IncludesOwnerName(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	_, err := q.CreateFeed(ctx, CreateFeedParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "HN", Url: "https://news.ycombinator.com/rss", UserID: user.ID})
	if err != nil {
		t.Fatalf("CreateFeed returned error: %v", err)
	}

	feeds, err := q.GetFeeds(ctx)
	if err != nil {
		t.Fatalf("GetFeeds returned error: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("len(feeds) = %d, want 1", len(feeds))
	}
	if feeds[0].UserName != "alice" {
		t.Errorf("UserName = %q, want %q", feeds[0].UserName, "alice")
	}
}

func TestMarkFeedFetched(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	feed, err := q.CreateFeed(ctx, CreateFeedParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "HN", Url: "https://news.ycombinator.com/rss", UserID: user.ID})
	if err != nil {
		t.Fatalf("CreateFeed returned error: %v", err)
	}

	updated, err := q.MarkFeedFetched(ctx, feed.ID)
	if err != nil {
		t.Fatalf("MarkFeedFetched returned error: %v", err)
	}
	if !updated.LastFetchedAt.Valid {
		t.Error("LastFetchedAt should be set after MarkFeedFetched")
	}
}

// getNextFeedToFetch is a test helper wrapping GetNextFeedsToFetch(ctx, 1)
// for the many existing tests that only care about a single result.
func getNextFeedToFetch(t *testing.T, q *Queries) Feed {
	t.Helper()
	feeds, err := q.GetNextFeedsToFetch(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetNextFeedsToFetch returned error: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("GetNextFeedsToFetch(limit=1) returned %d feeds, want 1", len(feeds))
	}
	return feeds[0]
}

func TestGetNextFeedsToFetch_NullsFirstThenOldest(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	feedA, err := q.CreateFeed(ctx, CreateFeedParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "A", Url: "https://a.example.com/rss", UserID: user.ID})
	if err != nil {
		t.Fatalf("CreateFeed(A) returned error: %v", err)
	}
	feedB, err := q.CreateFeed(ctx, CreateFeedParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "B", Url: "https://b.example.com/rss", UserID: user.ID})
	if err != nil {
		t.Fatalf("CreateFeed(B) returned error: %v", err)
	}

	// Neither feed has been fetched yet: either may come back first since
	// both have a NULL last_fetched_at, but the query must return one of them.
	next := getNextFeedToFetch(t, q)
	if next.ID != feedA.ID && next.ID != feedB.ID {
		t.Fatalf("GetNextFeedsToFetch returned unexpected feed %v", next.ID)
	}

	// Mark whichever feed came back as fetched; the other (still NULL)
	// must now be the next one up, regardless of creation order.
	if _, err := q.MarkFeedFetched(ctx, next.ID); err != nil {
		t.Fatalf("MarkFeedFetched returned error: %v", err)
	}

	after := getNextFeedToFetch(t, q)
	if after.ID == next.ID {
		t.Errorf("GetNextFeedsToFetch returned the just-fetched feed again; want the still-unfetched one")
	}
}

func TestGetNextFeedsToFetch_RespectsLimitAndOrder(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	now := time.Now().UTC()

	var created []Feed
	for _, name := range []string{"A", "B", "C"} {
		f, err := q.CreateFeed(ctx, CreateFeedParams{
			ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
			Name: name, Url: "https://" + name + ".example.com/rss", UserID: user.ID,
		})
		if err != nil {
			t.Fatalf("CreateFeed(%q) returned error: %v", name, err)
		}
		created = append(created, f)
		// Space out claims so ordering is deterministic (oldest first).
		if _, err := q.MarkFeedFetched(ctx, f.ID); err != nil {
			t.Fatalf("MarkFeedFetched(%q) returned error: %v", name, err)
		}
	}

	feeds, err := q.GetNextFeedsToFetch(ctx, 2)
	if err != nil {
		t.Fatalf("GetNextFeedsToFetch returned error: %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("len(feeds) = %d, want 2 (limit should be respected)", len(feeds))
	}
	if feeds[0].ID != created[0].ID || feeds[1].ID != created[1].ID {
		t.Errorf("GetNextFeedsToFetch did not return the two oldest-fetched feeds in order")
	}
}

func TestMarkFeedFetchSuccess_ResetsFailures(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")

	if _, err := q.MarkFeedFetchFailure(ctx, MarkFeedFetchFailureParams{ID: feed.ID, LastFetchError: "boom"}); err != nil {
		t.Fatalf("MarkFeedFetchFailure returned error: %v", err)
	}

	succeeded, err := q.MarkFeedFetchSuccess(ctx, feed.ID)
	if err != nil {
		t.Fatalf("MarkFeedFetchSuccess returned error: %v", err)
	}
	if succeeded.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 after MarkFeedFetchSuccess", succeeded.ConsecutiveFailures)
	}
	if succeeded.LastFetchError.Valid {
		t.Error("LastFetchError should be cleared after MarkFeedFetchSuccess")
	}
}

func TestMarkFeedFetchFailure_IncrementsAndStoresError(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")

	first, err := q.MarkFeedFetchFailure(ctx, MarkFeedFetchFailureParams{ID: feed.ID, LastFetchError: "timeout"})
	if err != nil {
		t.Fatalf("first MarkFeedFetchFailure returned error: %v", err)
	}
	if first.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1 after first failure", first.ConsecutiveFailures)
	}
	if !first.LastFetchError.Valid || first.LastFetchError.String != "timeout" {
		t.Errorf("LastFetchError = %+v, want valid %q", first.LastFetchError, "timeout")
	}

	second, err := q.MarkFeedFetchFailure(ctx, MarkFeedFetchFailureParams{ID: feed.ID, LastFetchError: "connection refused"})
	if err != nil {
		t.Fatalf("second MarkFeedFetchFailure returned error: %v", err)
	}
	if second.ConsecutiveFailures != 2 {
		t.Errorf("ConsecutiveFailures = %d, want 2 after second failure", second.ConsecutiveFailures)
	}
	if second.LastFetchError.String != "connection refused" {
		t.Errorf("LastFetchError = %q, want %q", second.LastFetchError.String, "connection refused")
	}
}

func TestGetNextFeedsToFetch_SkipsBackedOffFeed(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")

	unhealthy := createTestFeed(t, q, user.ID, "Unhealthy", "https://unhealthy.example.com/rss")
	healthy := createTestFeed(t, q, user.ID, "Healthy", "https://healthy.example.com/rss")

	// Claim both feeds (sets last_fetched_at to now for each), then record
	// a failure for one and a success for the other.
	if _, err := q.MarkFeedFetched(ctx, unhealthy.ID); err != nil {
		t.Fatalf("MarkFeedFetched(unhealthy) returned error: %v", err)
	}
	if _, err := q.MarkFeedFetchFailure(ctx, MarkFeedFetchFailureParams{ID: unhealthy.ID, LastFetchError: "boom"}); err != nil {
		t.Fatalf("MarkFeedFetchFailure returned error: %v", err)
	}
	if _, err := q.MarkFeedFetched(ctx, healthy.ID); err != nil {
		t.Fatalf("MarkFeedFetched(healthy) returned error: %v", err)
	}
	if _, err := q.MarkFeedFetchSuccess(ctx, healthy.ID); err != nil {
		t.Fatalf("MarkFeedFetchSuccess returned error: %v", err)
	}

	// Both feeds now have a recent last_fetched_at, but the unhealthy one
	// is within its backoff window (2 minutes after 1 failure) and must be
	// skipped in favor of the healthy one, which is always eligible.
	next := getNextFeedToFetch(t, q)
	if next.ID != healthy.ID {
		t.Errorf("GetNextFeedsToFetch returned %q, want the healthy feed %q", next.Name, healthy.Name)
	}
}

func TestGetNextFeedsToFetch_BackedOffFeedEligibleAfterWindow(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "Flaky", "https://flaky.example.com/rss")

	if _, err := q.MarkFeedFetched(ctx, feed.ID); err != nil {
		t.Fatalf("MarkFeedFetched returned error: %v", err)
	}
	if _, err := q.MarkFeedFetchFailure(ctx, MarkFeedFetchFailureParams{ID: feed.ID, LastFetchError: "boom"}); err != nil {
		t.Fatalf("MarkFeedFetchFailure returned error: %v", err)
	}

	// Simulate the backoff window (2 minutes for 1 failure) having elapsed.
	if _, err := q.db.ExecContext(ctx, `UPDATE feeds SET last_fetched_at = NOW() - INTERVAL '5 minutes' WHERE id = $1`, feed.ID); err != nil {
		t.Fatalf("simulating elapsed backoff window: %v", err)
	}

	next := getNextFeedToFetch(t, q)
	if next.ID != feed.ID {
		t.Errorf("GetNextFeedsToFetch did not return the feed once its backoff window elapsed")
	}
}
