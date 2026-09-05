-- name: MarkPostRead :one
INSERT INTO post_reads (id, created_at, updated_at, user_id, post_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, post_id) DO NOTHING
RETURNING *;

-- name: MarkPostUnread :exec
DELETE FROM post_reads WHERE user_id = $1 AND post_id = $2;
