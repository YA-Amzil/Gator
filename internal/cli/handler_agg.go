package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
	"gator/internal/rss"
)

// pubDateLayouts covers the date formats commonly found in RSS <pubDate>
// elements (RFC 822 variants, and a few ISO-8601-ish fallbacks).
var pubDateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"2006-01-02T15:04:05Z07:00",
	"Mon, 2 Jan 2006 15:04:05 -0700",
}

// fetchTimeout bounds how long a single feed's HTTP fetch may take. Without
// it, a server that hangs mid-response (rather than erroring outright) would
// block its goroutine forever, and since scrapeFeeds waits on every
// goroutine in the batch, one stuck feed could stall the next tick
// indefinitely. It's a var, not a const, so tests can shrink it instead of
// sleeping for the production value.
var fetchTimeout = 30 * time.Second

func HandlerAgg(s *State, cmd Command) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf("usage: agg <time_between_reqs> <concurrency>")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", cmd.Args[0], err)
	}

	concurrency, err := strconv.Atoi(cmd.Args[1])
	if err != nil || concurrency < 1 {
		return fmt.Errorf("invalid concurrency %q: must be a positive integer", cmd.Args[1])
	}

	fmt.Printf("Collecting feeds every %s (%d at a time)\n", timeBetweenRequests, concurrency)

	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		scrapeFeeds(s, concurrency)
	}
}

// scrapeFeeds fetches up to concurrency of the least-recently-fetched
// eligible feeds and processes them in parallel, one goroutine per feed, so
// a slow or failing feed doesn't hold up the others.
func scrapeFeeds(s *State, concurrency int) {
	ctx := context.Background()

	feeds, err := s.DB.GetNextFeedsToFetch(ctx, int32(concurrency))
	if err != nil {
		log.Printf("couldn't get next feeds to fetch: %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, feed := range feeds {
		wg.Add(1)
		go func(feed database.Feed) {
			defer wg.Done()
			scrapeFeed(ctx, s.DB, feed)
		}(feed)
	}
	wg.Wait()
}

// scrapeFeed marks a single feed fetched, downloads its RSS content, records
// the outcome (success clears its failure streak, failure backs it off),
// and saves any new posts. Output is buffered and printed in one call so
// concurrent feeds' lines don't interleave.
func scrapeFeed(ctx context.Context, db *database.Queries, feed database.Feed) {
	if _, err := db.MarkFeedFetched(ctx, feed.ID); err != nil {
		log.Printf("couldn't mark feed %q fetched: %v", feed.Name, err)
		return
	}

	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	rssFeed, err := rss.FetchFeed(fetchCtx, feed.Url)
	if err != nil {
		failed, markErr := db.MarkFeedFetchFailure(ctx, database.MarkFeedFetchFailureParams{
			ID:             feed.ID,
			LastFetchError: err.Error(),
		})
		if markErr != nil {
			log.Printf("couldn't record fetch failure for feed %q: %v", feed.Name, markErr)
		}
		log.Printf("couldn't fetch feed %q (%d consecutive failures): %v", feed.Name, failed.ConsecutiveFailures, err)
		return
	}

	if _, err := db.MarkFeedFetchSuccess(ctx, feed.ID); err != nil {
		log.Printf("couldn't record fetch success for feed %q: %v", feed.Name, err)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Fetched %d posts from %s\n", len(rssFeed.Channel.Item), feed.Name)
	for _, item := range rssFeed.Channel.Item {
		fmt.Fprintf(&out, "* %s\n", item.Title)
		savePost(ctx, db, feed.ID, item)
	}
	fmt.Print(out.String())
}

func savePost(ctx context.Context, db *database.Queries, feedID uuid.UUID, item rss.RSSItem) {
	now := time.Now().UTC()

	var publishedAt sql.NullTime
	if t, err := parsePubDate(item.PubDate); err == nil {
		publishedAt = sql.NullTime{Time: t, Valid: true}
	}

	_, err := db.CreatePost(ctx, database.CreatePostParams{
		ID:          uuid.New(),
		CreatedAt:   now,
		UpdatedAt:   now,
		Title:       item.Title,
		Url:         item.Link,
		Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
		PublishedAt: publishedAt,
		FeedID:      feedID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return // post already exists (unique url conflict), nothing to do
		}
		log.Printf("couldn't save post %q: %v", item.Title, err)
	}
}

func parsePubDate(raw string) (time.Time, error) {
	var lastErr error
	for _, layout := range pubDateLayouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("empty pubDate")
	}
	return time.Time{}, lastErr
}
