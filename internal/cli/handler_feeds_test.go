package cli

import (
	"context"
	"testing"

	"gator/internal/database"
)

func registerAndFetch(t *testing.T, s *State, name string) database.User {
	t.Helper()
	if err := HandlerRegister(s, Command{Args: []string{name}}); err != nil {
		t.Fatalf("HandlerRegister(%q) returned error: %v", name, err)
	}
	user, err := s.DB.GetUserByName(context.Background(), name)
	if err != nil {
		t.Fatalf("GetUserByName(%q) returned error: %v", name, err)
	}
	return user
}

func TestHandlerAddFeed_CreatesFeedAndAutoFollows(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice)
	if err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("expected feed to exist: %v", err)
	}
	if feed.Name != "Hacker News" {
		t.Errorf("feed.Name = %q, want %q", feed.Name, "Hacker News")
	}

	follows, err := s.DB.GetFeedFollowsForUser(context.Background(), alice.ID)
	if err != nil {
		t.Fatalf("GetFeedFollowsForUser returned error: %v", err)
	}
	if len(follows) != 1 {
		t.Fatalf("len(follows) = %d, want 1 (addfeed should auto-follow)", len(follows))
	}
	if follows[0].FeedName != "Hacker News" {
		t.Errorf("follows[0].FeedName = %q, want %q", follows[0].FeedName, "Hacker News")
	}
}

func TestHandlerAddFeed_WrongArgCount(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"only-one-arg"}}, alice); err == nil {
		t.Fatal("expected an error with one argument, got nil")
	}
}

func TestHandlerFeeds_ListsAllFeeds(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	if err := HandlerFeeds(s, Command{}); err != nil {
		t.Fatalf("HandlerFeeds returned error: %v", err)
	}
}

func TestHandlerFeeds_ShowsUnhealthyFeeds(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}
	feed, err := s.DB.GetFeedByURL(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	if _, err := s.DB.MarkFeedFetchFailure(context.Background(), database.MarkFeedFetchFailureParams{
		ID:             feed.ID,
		LastFetchError: "connection refused",
	}); err != nil {
		t.Fatalf("MarkFeedFetchFailure returned error: %v", err)
	}

	// HandlerFeeds should still succeed and print the unhealthy feed's status
	// without erroring; the exact formatting is covered by manual review of
	// handler_feeds.go, not asserted on stdout here.
	if err := HandlerFeeds(s, Command{}); err != nil {
		t.Fatalf("HandlerFeeds returned error: %v", err)
	}
}
