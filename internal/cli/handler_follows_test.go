package cli

import (
	"context"
	"testing"
)

func TestHandlerFollow_FollowsExistingFeed(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")
	bob := registerAndFetch(t, s, "bob")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	if err := HandlerFollow(s, Command{Args: []string{"https://news.ycombinator.com/rss"}}, bob); err != nil {
		t.Fatalf("HandlerFollow returned error: %v", err)
	}

	follows, err := s.DB.GetFeedFollowsForUser(context.Background(), bob.ID)
	if err != nil {
		t.Fatalf("GetFeedFollowsForUser returned error: %v", err)
	}
	if len(follows) != 1 {
		t.Fatalf("len(follows) = %d, want 1", len(follows))
	}
}

func TestHandlerFollow_UnknownURLFails(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerFollow(s, Command{Args: []string{"https://does-not-exist.example.com/rss"}}, alice); err == nil {
		t.Fatal("expected an error following a nonexistent feed URL, got nil")
	}
}

func TestHandlerFollowing_ListsFollowedFeeds(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	if err := HandlerFollowing(s, Command{}, alice); err != nil {
		t.Fatalf("HandlerFollowing returned error: %v", err)
	}
}

func TestHandlerUnfollow_RemovesFollow(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerAddFeed(s, Command{Args: []string{"Hacker News", "https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	if err := HandlerUnfollow(s, Command{Args: []string{"https://news.ycombinator.com/rss"}}, alice); err != nil {
		t.Fatalf("HandlerUnfollow returned error: %v", err)
	}

	follows, err := s.DB.GetFeedFollowsForUser(context.Background(), alice.ID)
	if err != nil {
		t.Fatalf("GetFeedFollowsForUser returned error: %v", err)
	}
	if len(follows) != 0 {
		t.Errorf("len(follows) = %d, want 0 after unfollow", len(follows))
	}
}

func TestHandlerUnfollow_UnknownURLFails(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	if err := HandlerUnfollow(s, Command{Args: []string{"https://does-not-exist.example.com/rss"}}, alice); err == nil {
		t.Fatal("expected an error unfollowing a nonexistent feed URL, got nil")
	}
}
