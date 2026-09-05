-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (url) DO NOTHING
RETURNING *;

-- name: GetPostsForUser :many
SELECT posts.*, (post_reads.id IS NOT NULL) AS is_read
FROM posts
INNER JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
LEFT JOIN post_reads ON post_reads.post_id = posts.id AND post_reads.user_id = $1
WHERE feed_follows.user_id = $1
ORDER BY posts.published_at DESC NULLS LAST
LIMIT $2;

-- name: GetPostByURL :one
SELECT * FROM posts WHERE url = $1;

-- name: CountUnreadPostsForUser :one
SELECT COUNT(*) FROM posts
INNER JOIN feed_follows ON posts.feed_id = feed_follows.feed_id
LEFT JOIN post_reads ON post_reads.post_id = posts.id AND post_reads.user_id = $1
WHERE feed_follows.user_id = $1 AND post_reads.id IS NULL;
