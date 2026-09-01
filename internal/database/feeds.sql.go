package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createFeed = `INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at, updated_at, name, url, user_id, last_fetched_at`

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
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt)
	return f, err
}

const getFeeds = `SELECT feeds.id, feeds.created_at, feeds.updated_at, feeds.name, feeds.url, feeds.user_id, feeds.last_fetched_at, users.name AS user_name
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
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt, &f.UserName); err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return feeds, nil
}

const getFeedByURL = `SELECT id, created_at, updated_at, name, url, user_id, last_fetched_at FROM feeds WHERE url = $1`

func (q *Queries) GetFeedByURL(ctx context.Context, url string) (Feed, error) {
	row := q.db.QueryRowContext(ctx, getFeedByURL, url)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt)
	return f, err
}

const markFeedFetched = `UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING id, created_at, updated_at, name, url, user_id, last_fetched_at`

func (q *Queries) MarkFeedFetched(ctx context.Context, id uuid.UUID) (Feed, error) {
	row := q.db.QueryRowContext(ctx, markFeedFetched, id)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt)
	return f, err
}

const getNextFeedToFetch = `SELECT id, created_at, updated_at, name, url, user_id, last_fetched_at
FROM feeds
ORDER BY last_fetched_at NULLS FIRST
LIMIT 1`

func (q *Queries) GetNextFeedToFetch(ctx context.Context) (Feed, error) {
	row := q.db.QueryRowContext(ctx, getNextFeedToFetch)
	var f Feed
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.Url, &f.UserID, &f.LastFetchedAt)
	return f, err
}
