package cli

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

func TestHandlerBrowse_DefaultLimit(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}
	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		_, err := s.DB.CreatePost(context.Background(), database.CreatePostParams{
			ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
			Title: "Post", Url: "https://news.ycombinator.com/" + uuid.NewString(),
			PublishedAt: sql.NullTime{Time: now.Add(time.Duration(i) * time.Minute), Valid: true},
			FeedID:      feed.ID,
		})
		if err != nil {
			t.Fatalf("CreatePost returned error: %v", err)
		}
	}

	// No explicit limit given: HandlerBrowse should fall back to its default of 2.
	if err := HandlerBrowse(s, Command{Args: []string{}}, alice); err != nil {
		t.Fatalf("HandlerBrowse returned error: %v", err)
	}
}

func TestHandlerBrowse_InvalidLimitFails(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerBrowse(s, Command{Args: []string{"not-a-number"}}, alice); err == nil {
		t.Fatal("expected an error for a non-numeric limit, got nil")
	}
}

func TestHandlerBrowse_NoPostsDoesNotError(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerBrowse(s, Command{Args: []string{}}, alice); err != nil {
		t.Fatalf("HandlerBrowse returned error: %v", err)
	}
}

func addTestPost(t *testing.T, s *State, feedID uuid.UUID, url string) database.Post {
	t.Helper()
	now := time.Now().UTC()
	p, err := s.DB.CreatePost(context.Background(), database.CreatePostParams{
		ID: uuid.New(), CreatedAt: now, UpdatedAt: now,
		Title: "Post", Url: url, FeedID: feedID,
	})
	if err != nil {
		t.Fatalf("CreatePost returned error: %v", err)
	}
	return p
}

func TestHandlerRead_MarksPostRead(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}
	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	post := addTestPost(t, s, feed.ID, "https://news.ycombinator.com/1")

	if err := HandlerRead(s, Command{Args: []string{post.Url}}, alice); err != nil {
		t.Fatalf("HandlerRead returned error: %v", err)
	}

	posts, err := s.DB.GetPostsForUser(context.Background(), database.GetPostsForUserParams{UserID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 1 || !posts[0].IsRead {
		t.Fatalf("posts = %+v, want exactly one post marked read", posts)
	}
}

func TestHandlerRead_AlreadyReadDoesNotError(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}
	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	post := addTestPost(t, s, feed.ID, "https://news.ycombinator.com/1")

	if err := HandlerRead(s, Command{Args: []string{post.Url}}, alice); err != nil {
		t.Fatalf("first HandlerRead returned error: %v", err)
	}
	if err := HandlerRead(s, Command{Args: []string{post.Url}}, alice); err != nil {
		t.Fatalf("second HandlerRead returned error: %v, want nil (already-read is a no-op)", err)
	}
}

func TestHandlerRead_UnknownURLFails(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerRead(s, Command{Args: []string{"https://does-not-exist.example.com/1"}}, alice); err == nil {
		t.Fatal("expected an error for an unknown post url, got nil")
	}
}

func TestHandlerUnread_RemovesReadStatus(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}
	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	post := addTestPost(t, s, feed.ID, "https://news.ycombinator.com/1")

	if err := HandlerRead(s, Command{Args: []string{post.Url}}, alice); err != nil {
		t.Fatalf("HandlerRead returned error: %v", err)
	}
	if err := HandlerUnread(s, Command{Args: []string{post.Url}}, alice); err != nil {
		t.Fatalf("HandlerUnread returned error: %v", err)
	}

	posts, err := s.DB.GetPostsForUser(context.Background(), database.GetPostsForUserParams{UserID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 1 || posts[0].IsRead {
		t.Fatalf("posts = %+v, want exactly one post marked unread", posts)
	}
}

func TestHandlerUnread_UnknownURLFails(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerUnread(s, Command{Args: []string{"https://does-not-exist.example.com/1"}}, alice); err == nil {
		t.Fatal("expected an error for an unknown post url, got nil")
	}
}
