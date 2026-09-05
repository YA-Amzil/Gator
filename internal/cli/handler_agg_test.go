package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	scrapeFeeds(s, 1)

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

	scrapeFeeds(s, 1)

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

func TestScrapeFeeds_FetchesBatchConcurrently(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	// Each server must return a distinct item link, or the second post would
	// silently collide with the first on posts.url (ON CONFLICT DO NOTHING)
	// and mask a real concurrency bug behind what looks like a fixture bug.
	okServer := func(postLink string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A feed for tests</description>
    <item>
      <title>Post from %s</title>
      <link>%s</link>
      <description>Body</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
  </channel>
</rss>`, postLink, postLink)
		}))
	}
	brokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer brokenServer.Close()

	serverA := okServer("https://example.com/a")
	serverB := okServer("https://example.com/b")
	defer serverA.Close()
	defer serverB.Close()

	names := []string{"Feed A", "Feed B", "Feed C (broken)"}
	urls := []string{serverA.URL, serverB.URL, brokenServer.URL}
	for i := range names {
		if err := HandlerAddFeed(s, Command{Args: []string{names[i], urls[i]}}, alice); err != nil {
			t.Fatalf("HandlerAddFeed(%q) returned error: %v", names[i], err)
		}
	}

	// concurrency=3 should claim and process all three feeds in one call,
	// each in its own goroutine, without misattributing results between them.
	scrapeFeeds(s, 3)

	feedA, err := s.DB.GetFeedByURL(context.Background(), serverA.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL(A) returned error: %v", err)
	}
	feedB, err := s.DB.GetFeedByURL(context.Background(), serverB.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL(B) returned error: %v", err)
	}
	feedC, err := s.DB.GetFeedByURL(context.Background(), brokenServer.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL(C) returned error: %v", err)
	}

	for name, f := range map[string]database.Feed{"A": feedA, "B": feedB} {
		if !f.LastFetchedAt.Valid {
			t.Errorf("feed %s: LastFetchedAt should be set", name)
		}
		if f.ConsecutiveFailures != 0 {
			t.Errorf("feed %s: ConsecutiveFailures = %d, want 0", name, f.ConsecutiveFailures)
		}
	}
	if feedC.ConsecutiveFailures != 1 {
		t.Errorf("feed C: ConsecutiveFailures = %d, want 1", feedC.ConsecutiveFailures)
	}

	postsA, err := s.DB.GetPostsForUser(context.Background(), database.GetPostsForUserParams{UserID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatalf("GetPostsForUser returned error: %v", err)
	}
	if len(postsA) != 2 {
		t.Fatalf("len(posts) = %d, want 2 (one from each successful feed, none from the broken one)", len(postsA))
	}
	for _, p := range postsA {
		if p.FeedID != feedA.ID && p.FeedID != feedB.ID {
			t.Errorf("post %q attributed to unexpected feed %v (concurrency may have mixed up feed IDs)", p.Title, p.FeedID)
		}
	}
}

func TestScrapeFeeds_TimeoutMarksFailureAndDoesNotHang(t *testing.T) {
	s := newTestState(t)
	alice := registerAndFetch(t, s, "alice")

	// Shrink fetchTimeout for the test instead of waiting out the 30s
	// production value; restored so it can't leak into other tests.
	original := fetchTimeout
	fetchTimeout = 50 * time.Millisecond
	defer func() { fetchTimeout = original }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // much longer than fetchTimeout
		w.Write([]byte(testFeedXML))
	}))
	defer server.Close()

	if err := HandlerAddFeed(s, Command{Args: []string{"Slow Feed", server.URL}}, alice); err != nil {
		t.Fatalf("HandlerAddFeed returned error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		scrapeFeeds(s, 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scrapeFeeds did not return within 2s; a hanging fetch is blocking the whole batch")
	}

	feed, err := s.DB.GetFeedByURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("GetFeedByURL returned error: %v", err)
	}
	if feed.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1 after a timed-out fetch", feed.ConsecutiveFailures)
	}
	if !feed.LastFetchError.Valid || feed.LastFetchError.String == "" {
		t.Error("LastFetchError should be set after a timed-out fetch")
	}
}
