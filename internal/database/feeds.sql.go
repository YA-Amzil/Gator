package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const feedColumns = "id, created_at, updated_at, name, url, user_id, last_fetched_at, consecutive_failures, last_fetch_error"

const createFeed = `INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING ` + feedColumns

type CreateFeedParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Url       string
	UserID    uuid.UUID
}

func (q *Queries) CreateFeed(ctx context.Context, arg CreateFeedParams) (Feed, error) {
	row := q.db.QueryRowContext(ctx, createFeed, arg.ID, arg.CreatedAt, arg.UpdatedAt, arg.Name, arg.Url, arg.UserID)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError)
	return f, err
}

const getFeeds = `SELECT feeds.id, feeds.created_at, feeds.updated_at, feeds.name, feeds.url, feeds.user_id,
       feeds.last_fetched_at, feeds.consecutive_failures, feeds.last_fetch_error, users.name AS user_name
FROM feeds
INNER JOIN users ON feeds.user_id = users.id
ORDER BY feeds.created_at`

func (q *Queries) GetFeeds(ctx context.Context) ([]FeedWithUser, error) {
	rows, err := q.db.QueryContext(ctx, getFeeds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []FeedWithUser
	for rows.Next() {
		var f FeedWithUser
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError, &f.UserName); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return feeds, nil
}

const getFeedByURL = `SELECT ` + feedColumns + ` FROM feeds WHERE url = $1`

func (q *Queries) GetFeedByURL(ctx context.Context, url string) (Feed, error) {
	row := q.db.QueryRowContext(ctx, getFeedByURL, url)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError)
	return f, err
}

const markFeedFetched = `UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING ` + feedColumns

func (q *Queries) MarkFeedFetched(ctx context.Context, id uuid.UUID) (Feed, error) {
	row := q.db.QueryRowContext(ctx, markFeedFetched, id)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError)
	return f, err
}

// getNextFeedsToFetch: healthy feeds (no consecutive failures) are always
// eligible. A feed with consecutive failures backs off exponentially
// (2, 4, 8, ... minutes, capped at 60) since its last fetch attempt before
// it's tried again. Returns up to `limit` feeds so callers can fetch a batch
// concurrently.
const getNextFeedsToFetch = `SELECT ` + feedColumns + `
FROM feeds
WHERE consecutive_failures = 0
   OR last_fetched_at IS NULL
   OR last_fetched_at <= NOW() - (INTERVAL '1 minute' * LEAST(POWER(2, consecutive_failures), 60))
ORDER BY last_fetched_at NULLS FIRST
LIMIT $1`

func (q *Queries) GetNextFeedsToFetch(ctx context.Context, limit int32) ([]Feed, error) {
	rows, err := q.db.QueryContext(ctx, getNextFeedsToFetch, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return feeds, nil
}

const markFeedFetchSuccess = `UPDATE feeds
SET consecutive_failures = 0, last_fetch_error = NULL, updated_at = NOW()
WHERE id = $1
RETURNING ` + feedColumns

// MarkFeedFetchSuccess clears a feed's failure streak after a successful fetch.
func (q *Queries) MarkFeedFetchSuccess(ctx context.Context, id uuid.UUID) (Feed, error) {
	row := q.db.QueryRowContext(ctx, markFeedFetchSuccess, id)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError)
	return f, err
}

const markFeedFetchFailure = `UPDATE feeds
SET consecutive_failures = consecutive_failures + 1, last_fetch_error = $2, updated_at = NOW()
WHERE id = $1
RETURNING ` + feedColumns

type MarkFeedFetchFailureParams struct {
	ID             uuid.UUID
	LastFetchError string
}

// MarkFeedFetchFailure records a failed fetch attempt, incrementing the
// feed's consecutive failure streak so GetNextFeedToFetch backs off it.
func (q *Queries) MarkFeedFetchFailure(ctx context.Context, arg MarkFeedFetchFailureParams) (Feed, error) {
	row := q.db.QueryRowContext(ctx, markFeedFetchFailure, arg.ID, arg.LastFetchError)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.ConsecutiveFailures, &f.LastFetchError)
	return f, err
}
