package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createTestPost(t *testing.T, q *Queries, feedID uuid.UUID, title, url string) Post {
	t.Helper()
	now := time.Now().UTC()
	p, err := q.CreatePost(context.Background(), CreatePostParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
		Title: title, Url: url, FeedID: feedID,
	})
	if err != nil {
		t.Fatalf("createTestPost(%q): %v", title, err)
	}
	return p
}

func TestMarkPostRead(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")
	post := createTestPost(t, q, feed.ID, "Show HN: Gator", "https://news.ycombinator.com/item?id=1")
	now := time.Now().UTC()

	read, err := q.MarkPostRead(ctx, MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: post.ID})
	if err != nil {
		t.Fatalf("MarkPostRead returned error: %v", err)
	}
	if read.UserID != user.ID || read.PostID != post.ID {
		t.Errorf("MarkPostRead returned unexpected row: %+v", read)
	}
}

func TestMarkPostRead_DuplicateIsNoRows(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")
	post := createTestPost(t, q, feed.ID, "Show HN: Gator", "https://news.ycombinator.com/item?id=1")
	now := time.Now().UTC()

	params := MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: post.ID}
	if _, err := q.MarkPostRead(ctx, params); err != nil {
		t.Fatalf("first MarkPostRead returned error: %v", err)
	}

	params.ID = uuid.New()
	_, err := q.MarkPostRead(ctx, params)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second MarkPostRead error = %v, want sql.ErrNoRows (marking an already-read post should be a silent no-op)", err)
	}
}

func TestMarkPostUnread_RemovesReadRecord(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")
	post := createTestPost(t, q, feed.ID, "Show HN: Gator", "https://news.ycombinator.com/item?id=1")
	now := time.Now().UTC()

	if _, err := q.MarkPostRead(ctx, MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: post.ID}); err != nil {
		t.Fatalf("MarkPostRead returned error: %v", err)
	}

	if err := q.MarkPostUnread(ctx, MarkPostUnreadParams{UserID: user.ID, PostID: post.ID}); err != nil {
		t.Fatalf("MarkPostUnread returned error: %v", err)
	}

	// Marking read again should succeed (no longer a duplicate), proving the
	// row was actually deleted rather than left behind.
	if _, err := q.MarkPostRead(ctx, MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: post.ID}); err != nil {
		t.Errorf("MarkPostRead after unread returned error: %v, want nil", err)
	}
}

func TestGetPostsForUser_ReflectsReadStatus(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")
	if _, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), UserID: user.ID, FeedID: feed.ID}); err != nil {
		t.Fatalf("CreateFeedFollow returned error: %v", err)
	}

	readPost := createTestPost(t, q, feed.ID, "Read post", "https://news.ycombinator.com/1")
	unreadPost := createTestPost(t, q, feed.ID, "Unread post", "https://news.ycombinator.com/2")

	now := time.Now().UTC()
	if _, err := q.MarkPostRead(ctx, MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: readPost.ID}); err != nil {
		t.Fatalf("MarkPostRead returned error: %v", err)
	}

	posts, err := q.GetPostsForUser(ctx, GetPostsForUserParams{UserID: user.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}

	for _, p := range posts {
		switch p.ID {
		case readPost.ID:
			if !p.IsRead {
				t.Errorf("post %q: IsRead = false, want true", p.Title)
			}
		case unreadPost.ID:
			if p.IsRead {
				t.Errorf("post %q: IsRead = true, want false", p.Title)
			}
		default:
			t.Errorf("unexpected post %q", p.Title)
		}
	}
}

func TestCountUnreadPostsForUser(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	user := createTestUser(t, q, "alice")
	feed := createTestFeed(t, q, user.ID, "HN", "https://news.ycombinator.com/rss")
	if _, err := q.CreateFeedFollow(ctx, CreateFeedFollowParams{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), UserID: user.ID, FeedID: feed.ID}); err != nil {
		t.Fatalf("CreateFeedFollow returned error: %v", err)
	}

	var posts []Post
	for i := 0; i < 3; i++ {
		posts = append(posts, createTestPost(t, q, feed.ID, "Post", "https://news.ycombinator.com/"+uuid.NewString()))
	}

	now := time.Now().UTC()
	if _, err := q.MarkPostRead(ctx, MarkPostReadParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, UserID: user.ID, PostID: posts[0].ID}); err != nil {
		t.Fatalf("MarkPostRead returned error: %v", err)
	}

	count, err := q.CountUnreadPostsForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountUnreadPostsForUser returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("CountUnreadPostsForUser = %d, want 2", count)
	}
}
