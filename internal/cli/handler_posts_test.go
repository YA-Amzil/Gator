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
