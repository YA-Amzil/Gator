package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreatePost(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, owner.ID, "Hacker News", "https://news.ycombinator.com/rss")
	now := time.Now().UTC()

	post, err := q.CreatePost(ctx, CreatePostParams{
		ID:          uuid.New(),
		CreatedAt:   now,
		UpdatedAt:   now,
		Title:       "Show HN: Gator",
		Url:         "https://news.ycombinator.com/item?id=1",
		Description: sql.NullString{String: "A blog aggregator", Valid: true},
		PublishedAt: sql.NullTime{Time: now, Valid: true},
		FeedID:      feed.ID,
	})
	if err != nil {
		t.Fatalf("CreatePost returned error: %v", err)
	}
	if post.Title != "Show HN: Gator" {
		t.Errorf("Title = %q, want %q", post.Title, "Show HN: Gator")
	}
}

func TestCreatePost_DuplicateURLIsNoRows(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, owner.ID, "Hacker News", "https://news.ycombinator.com/rss")
	now := time.Now().UTC()

	params := CreatePostParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
		Title: "Show HN: Gator", Url: "https://news.ycombinator.com/item?id=1",
		FeedID: feed.ID,
	}
	if _, err := q.CreatePost(ctx, params); err != nil {
		t.Fatalf("first CreatePost returned error: %v", err)
	}

	params.ID = uuid.New()
	_, err := q.CreatePost(ctx, params)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second CreatePost error = %v, want sql.ErrNoRows (duplicate url should be a silent no-op)", err)
	}
}

func TestGetPostsForUser_OnlyFollowedFeedsAndRespectsLimit(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	owner := createTestUser(t, q, "alice")
	followedFeed := createTestFeed(t, q, owner.ID, "Followed", "https://followed.example.com/rss")
	otherFeed := createTestFeed(t, q, owner.ID, "Other", "https://other.example.com/rss")
	now := time.Now().UTC()

	if _, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: owner.ID, FeedID: followedFeed.ID}); err != nil {
		t.Fatalf("CreateFeedFollow returned error: %v", err)
	}

	// Three posts on the followed feed, one on a feed the user does not follow.
	for i, title := range []string{"Post 1", "Post 2", "Post 3"} {
		published := now.Add(time.Duration(i) * time.Hour)
		if _, err := q.CreatePost(ctx, CreatePostParams{
			ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
			Title: title, Url: "https://followed.example.com/" + title,
			PublishedAt: sql.NullTime{Time: published, Valid: true},
			FeedID:      followedFeed.ID,
		}); err != nil {
			t.Fatalf("CreatePost(%q) returned error: %v", title, err)
		}
	}
	if _, err := q.CreatePost(ctx, CreatePostParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
		Title: "Unfollowed post", Url: "https://other.example.com/1",
		FeedID: otherFeed.ID,
	}); err != nil {
		t.Fatalf("CreatePost(unfollowed) returned error: %v", err)
	}

	posts, err := q.GetPostsForUser(ctx, GetPostsForUserParams{UserID: owner.ID, Limit: 2})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2 (limit should be respected)", len(posts))
	}

	// Most recently published first.
	if posts[0].Title != "Post 3" {
		t.Errorf("posts[0].Title = %q, want %q (most recently published first)", posts[0].Title, "Post 3")
	}

	for _, p := range posts {
		if p.FeedID == otherFeed.ID {
			t.Errorf("GetPostsForUser returned a post from an unfollowed feed: %q", p.Title)
		}
	}
}
