package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleFeedXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example &amp; Co Blog</title>
    <link>https://example.com</link>
    <description>News &amp; updates</description>
    <item>
      <title>First &lt;post&gt;</title>
      <link>https://example.com/1</link>
      <description>Body &amp; more</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>Second post</title>
      <link>https://example.com/2</link>
      <description>Plain body</description>
      <pubDate>Tue, 03 Jan 2006 15:04:05 -0700</pubDate>
    </item>
  </channel>
</rss>`

func TestFetchFeed(t *testing.T) {
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleFeedXML))
	}))
	defer server.Close()

	feed, err := FetchFeed(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchFeed returned error: %v", err)
	}

	if gotUserAgent != "gator" {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, "gator")
	}

	if want := "Example & Co Blog"; feed.Channel.Title != want {
		t.Errorf("Channel.Title = %q, want %q", feed.Channel.Title, want)
	}
	if want := "News & updates"; feed.Channel.Description != want {
		t.Errorf("Channel.Description = %q, want %q", feed.Channel.Description, want)
	}

	if len(feed.Channel.Item) != 2 {
		t.Fatalf("len(Channel.Item) = %d, want 2", len(feed.Channel.Item))
	}

	first := feed.Channel.Item[0]
	if want := "First <post>"; first.Title != want {
		t.Errorf("Item[0].Title = %q, want %q", first.Title, want)
	}
	if want := "Body & more"; first.Description != want {
		t.Errorf("Item[0].Description = %q, want %q", first.Description, want)
	}
	if want := "https://example.com/1"; first.Link != want {
		t.Errorf("Item[0].Link = %q, want %q", first.Link, want)
	}

	second := feed.Channel.Item[1]
	if want := "Second post"; second.Title != want {
		t.Errorf("Item[1].Title = %q, want %q", second.Title, want)
	}
}

func TestFetchFeed_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := FetchFeed(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to mention the 404 status", err.Error())
	}
}

func TestFetchFeed_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not xml"))
	}))
	defer server.Close()

	_, err := FetchFeed(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected an error for invalid XML, got nil")
	}
}

func TestFetchFeed_InvalidURL(t *testing.T) {
	_, err := FetchFeed(context.Background(), "://not-a-url")
	if err == nil {
		t.Fatal("expected an error for an invalid URL, got nil")
	}
}
