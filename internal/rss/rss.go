package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
)

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// maxFeedBodySize caps how much of a feed response FetchFeed will buffer
// into memory. Without a cap, a malicious or just misbehaving feed URL could
// return an arbitrarily large (or infinite) body and exhaust memory —
// especially now that agg fetches multiple feeds concurrently, each
// buffering independently. Real RSS feeds are at most a few hundred KB; 10
// MiB is generous headroom. It's a var, not a const, so tests can shrink it
// instead of transferring megabytes of filler.
var maxFeedBodySize int64 = 10 << 20 // 10 MiB

// FetchFeed downloads and parses the RSS feed at feedURL, unescaping HTML
// entities in the channel and item title/description fields.
func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", feedURL, err)
	}
	req.Header.Set("User-Agent", "gator")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", feedURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %s", res.StatusCode, feedURL)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxFeedBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", feedURL, err)
	}
	if int64(len(data)) > maxFeedBodySize {
		return nil, fmt.Errorf("response from %s exceeds %d byte limit", feedURL, maxFeedBodySize)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parsing xml from %s: %w", feedURL, err)
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
}
