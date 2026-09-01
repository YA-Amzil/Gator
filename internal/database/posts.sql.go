package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

const createPost = `INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (url) DO NOTHING
RETURNING id, created_at, updated_at, title, url, description, published_at, feed_id`

type CreatePostParams struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Url         string
	Description sql.NullString
	PublishedAt sql.NullTime
	FeedID      uuid.UUID
}

// CreatePost inserts a post, skipping duplicates by URL. If the post already
// exists, it returns sql.ErrNoRows, which callers should treat as a no-op.
func (q *Queries) CreatePost(ctx context.Context, arg CreatePostParams) (Post, error) {
	row := q.db.QueryRowContext(ctx, createPost,
		arg.ID, arg.CreatedAt, arg.UpdatedAt, arg.Title, arg.Url, arg.Description, arg.PublishedAt, arg.FeedID,
	)
	var p Post
	err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt, &p.Title, &p.Url, &p.Description, &p.PublishedAt, &p.FeedID)
	return p, err
}

const getPostsForUser = `SELECT posts.id, posts.created_at, posts.updated_at, posts.title, posts.url,
       posts.description, posts.published_at, posts.feed_id
FROM posts
INNER JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
WHERE feed_follows.user_id = $1
ORDER BY posts.published_at DESC NULLS LAST
LIMIT $2`

type GetPostsForUserParams struct {
	UserID uuid.UUID
	Limit  int32
}

func (q *Queries) GetPostsForUser(ctx context.Context, arg GetPostsForUserParams) ([]Post, error) {
	rows, err := q.db.QueryContext(ctx, getPostsForUser, arg.UserID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt, &p.Title, &p.Url, &p.Description, &p.PublishedAt, &p.FeedID); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}
