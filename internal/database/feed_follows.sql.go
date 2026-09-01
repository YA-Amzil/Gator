package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createFeedFollow = `WITH inserted_feed_follow AS (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, created_at, updated_at, user_id, feed_id
)
SELECT inserted_feed_follow.id, inserted_feed_follow.created_at, inserted_feed_follow.updated_at,
       inserted_feed_follow.user_id, inserted_feed_follow.feed_id,
       feeds.name AS feed_name, users.name AS user_name
FROM inserted_feed_follow
INNER JOIN feeds ON inserted_feed_follow.feed_id = feeds.id
INNER JOIN users ON inserted_feed_follow.user_id = users.id`

type CreateFeedFollowParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uuid.UUID
	FeedID    uuid.UUID
}

func (q *Queries) CreateFeedFollow(ctx context.Context, arg CreateFeedFollowParams) (FeedFollowRow, error) {
	row := q.db.QueryRowContext(ctx, createFeedFollow, arg.ID, arg.CreatedAt, arg.UpdatedAt, arg.UserID, arg.FeedID)
	var f FeedFollowRow
	err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.UserID, &f.FeedID, &f.FeedName, &f.UserName)
	return f, err
}

const getFeedFollowsForUser = `SELECT feed_follows.id, feed_follows.created_at, feed_follows.updated_at,
       feed_follows.user_id, feed_follows.feed_id,
       feeds.name AS feed_name, users.name AS user_name
FROM feed_follows
INNER JOIN feeds ON feed_follows.feed_id = feeds.id
INNER JOIN users ON feed_follows.user_id = users.id
WHERE feed_follows.user_id = $1
ORDER BY feeds.name`

func (q *Queries) GetFeedFollowsForUser(ctx context.Context, userID uuid.UUID) ([]FeedFollowRow, error) {
	rows, err := q.db.QueryContext(ctx, getFeedFollowsForUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []FeedFollowRow
	for rows.Next() {
		var f FeedFollowRow
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.UserID, &f.FeedID, &f.FeedName, &f.UserName); err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return follows, nil
}

const deleteFeedFollow = `DELETE FROM feed_follows WHERE user_id = $1 AND feed_id = $2`

type DeleteFeedFollowParams struct {
	UserID uuid.UUID
	FeedID uuid.UUID
}

func (q *Queries) DeleteFeedFollow(ctx context.Context, arg DeleteFeedFollowParams) error {
	_, err := q.db.ExecContext(ctx, deleteFeedFollow, arg.UserID, arg.FeedID)
	return err
}
