package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gator/internal/database"
)

const testFeedXML = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A feed for tests</description>
    <item>
      <title>Test Post</title>
      <link>https://example.com/1</link>
      <description>Body</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
  </channel>
</rss>`

func TestScrapeFeeds_SuccessMarksHealthyAndSavesPost(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testFeedXML))
	}))
	defer server.Close()

	if err := HandlerAddFeed(s, Command{Args: []string{"Test Feed", server.URL}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	scrapeFeeds(s)

	feed, err := s.DB.GetFeedByURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	if feed.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 after a successful scrape", feed.ConsecutiveFailures)
	}
	if feed.LastFetchError.Valid {
		t.Errorf("LastFetchError = %q, want NULL after a successful scrape", feed.LastFetchError.String)
	}
	if !feed.LastFetchedAt.Valid {
		t.Error("LastFetchedAt should be set after scrapeFeeds runs")
	}

	posts, err := s.DB.GetPostsForUser(context.Background(), database.GetPostsForUserParams{UserID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(posts))
	}
	if posts[0].Title != "Test Post" {
		t.Errorf("posts[0].Title = %q, want %q", posts[0].Title, "Test Post")
	}
}

func TestScrapeFeeds_FailureMarksUnhealthy(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := HandlerAddFeed(s, Command{Args: []string{"Broken Feed", server.URL}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	scrapeFeeds(s)

	feed, err := s.DB.GetFeedByURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	if feed.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1 after a failed scrape", feed.ConsecutiveFailures)
	}
	if !feed.LastFetchError.Valid || feed.LastFetchError.String == "" {
		t.Error("LastFetchError should be set after a failed scrape")
	}

	posts, err := s.DB.GetPostsForUser(context.Background(), database.GetPostsForUserParams{UserID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("len(posts) = %d, want 0 after a failed scrape", len(posts))
	}
}
