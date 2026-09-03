package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

func HandlerAgg(s *State, cmd Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: agg <time_between_reqs>")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", cmd.Args[0], err)
	}

	fmt.Printf("Collecting feeds every %s\n", timeBetweenRequests)

	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

// scrapeFeeds fetches the least-recently-fetched feed, marks it fetched,
// downloads its RSS content, and saves any new posts.
func scrapeFeeds(s *State) {
	ctx := context.Background()

	feed, err := s.DB.GetNextFeedToFetch(ctx)
	if err != nil {
		log.Printf("couldn't get next feed to fetch: %v", err)
		return
	}

	if _, err := s.DB.MarkFeedFetched(ctx, feed.ID); err != nil {
		log.Printf("couldn't mark feed %q fetched: %v", feed.Name, err)
		return
	}

	rssFeed, err := rss.FetchFeed(ctx, feed.Url)
	if err != nil {
		failed, markErr := s.DB.MarkFeedFetchFailure(ctx, database.MarkFeedFetchFailureParams{
			ID:             feed.ID,
			LastFetchError: err.Error(),
		})
		if markErr != nil {
			log.Printf("couldn't record fetch failure for feed %q: %v", feed.Name, markErr)
		}
		log.Printf("couldn't fetch feed %q (%d consecutive failures): %v", feed.Name, failed.ConsecutiveFailures, err)
		return
	}

	if _, err := s.DB.MarkFeedFetchSuccess(ctx, feed.ID); err != nil {
		log.Printf("couldn't record fetch success for feed %q: %v", feed.Name, err)
	}

	fmt.Printf("Fetched %d posts from %s\n", len(rssFeed.Channel.Item), feed.Name)

	for _, item := range rssFeed.Channel.Item {
		fmt.Printf("* %s\n", item.Title)
		savePost(ctx, s.DB, feed.ID, item)
	}
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
