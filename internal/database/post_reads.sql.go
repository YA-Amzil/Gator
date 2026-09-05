package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const markPostRead = `INSERT INTO post_reads (id, created_at, updated_at, user_id, post_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, post_id) DO NOTHING
RETURNING id, created_at, updated_at, user_id, post_id`

type MarkPostReadParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uuid.UUID
	PostID    uuid.UUID
}

// MarkPostRead records that a user has read a post, skipping duplicates by
// (user_id, post_id). If the post is already marked read, it returns
// sql.ErrNoRows, which callers should treat as a no-op.
func (q *Queries) MarkPostRead(ctx context.Context, arg MarkPostReadParams) (PostRead, error) {
	row := q.db.QueryRowContext(ctx, markPostRead, arg.ID, arg.CreatedAt, arg.UpdatedAt, arg.UserID, arg.PostID)
	var r PostRead
	err := row.Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt, &r.UserID, &r.PostID)
	return r, err
}

const markPostUnread = `DELETE FROM post_reads WHERE user_id = $1 AND post_id = $2`

type MarkPostUnreadParams struct {
	UserID uuid.UUID
	PostID uuid.UUID
}

func (q *Queries) MarkPostUnread(ctx context.Context, arg MarkPostUnreadParams) error {
	_, err := q.db.ExecContext(ctx, markPostUnread, arg.UserID, arg.PostID)
	return err
}
