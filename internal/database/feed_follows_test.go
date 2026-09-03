package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createTestFeed(t *testing.T, q *Queries, ownerID uuid.UUID, name, url string) Feed {
	t.Helper()
	now := time.Now().UTC()
	f, err := q.CreateFeed(context.Background(), CreateFeedParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: name, Url: url, UserID: ownerID,
	})
	if err != nil {
		t.Fatalf("createTestFeed(%q): %v", name, err)
	}
	return f
}

func TestCreateFeedFollow_ReturnsJoinedNames(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	follower := createTestUser(t, q, "bob")
	feed := createTestFeed(t, q, owner.ID, "Hacker News", "https://news.ycombinator.com/rss")
	now := time.Now().UTC()

	follow, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: follower.ID, FeedID: feed.ID,
	})
	if err != nil {
		t.Fatalf("CreateFeedFollow returned error: %v", err)
	}
	if follow.FeedName != "Hacker News" {
		t.Errorf("FeedName = %q, want %q", follow.FeedName, "Hacker News")
	}
	if follow.UserName != "bob" {
		t.Errorf("UserName = %q, want %q", follow.UserName, "bob")
	}
}

func TestCreateFeedFollow_DuplicateFails(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, owner.ID, "Hacker News", "https://news.ycombinator.com/rss")
	now := time.Now().UTC()

	params := CreateFeedFollowParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: owner.ID, FeedID: feed.ID}
	if _, err := q.CreateFeedFollow(ctx, params); err != nil {
		t.Fatalf("first CreateFeedFollow returned error: %v", err)
	}

	params.ID = uuid.New()
	if _, err := q.CreateFeedFollow(ctx, params); err == nil {
		t.Fatal("expected an error following the same feed twice, got nil")
	}
}

func TestGetFeedFollowsForUser(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	feedA := createTestFeed(t, q, owner.ID, "A", "https://a.example.com/rss")
	feedB := createTestFeed(t, q, owner.ID, "B", "https://b.example.com/rss")
	now := time.Now().UTC()

	for _, f := range []Feed{feedA, feedB} {
		if _, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: owner.ID, FeedID: f.ID}); err != nil {
			t.Fatalf("CreateFeedFollow(%q) returned error: %v", f.Name, err)
		}
	}

	follows, err := q.GetFeedFollowsForUser(ctx, owner.ID)
	if err != nil {
		t.Fatalf("GetFeedFollowsForUser returned error: %v", err)
	}
	if len(follows) != 2 {
		t.Fatalf("len(follows) = %d, want 2", len(follows))
	}
}

func TestDeleteFeedFollow(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, owner.ID, "Hacker News", "https://news.ycombinator.com/rss")
	now := time.Now().UTC()

	if _, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: owner.ID, FeedID: feed.ID}); err != nil {
		t.Fatalf("CreateFeedFollow returned error: %v", err)
	}

	if err := q.DeleteFeedFollow(ctx, DeleteFeedFollowParams{UserID: owner.ID, FeedID: feed.ID}); err != nil {
		t.Fatalf("DeleteFeedFollow returned error: %v", err)
	}

	follows, err := q.GetFeedFollowsForUser(ctx, owner.ID)
	if err != nil {
		t.Fatalf("GetFeedFollowsForUser returned error: %v", err)
	}
	if len(follows) != 0 {
		t.Errorf("len(follows) = %d, want 0 after DeleteFeedFollow", len(follows))
	}
}
