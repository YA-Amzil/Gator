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

func TestGetNextFeedToFetch_NullsFirstThenOldest(t *testing.T) {
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
	next, err := q.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("GetNextFeedToFetch returned error: %v", err)
	}
	if next.ID != feedA.ID && next.ID != feedB.ID {
		t.Fatalf("GetNextFeedToFetch returned unexpected feed %v", next.ID)
	}

	// Mark whichever feed came back as fetched; the other (still NULL)
	// must now be the next one up, regardless of creation order.
	if _, err := q.MarkFeedFetched(ctx, next.ID); err != nil {
		t.Fatalf("MarkFeedFetched returned error: %v", err)
	}

	after, err := q.GetNextFeedToFetch(ctx)
	if err != nil {
		t.Fatalf("GetNextFeedToFetch returned error: %v", err)
	}
	if after.ID == next.ID {
		t.Errorf("GetNextFeedToFetch returned the just-fetched feed again; want the still-unfetched one")
	}
}
