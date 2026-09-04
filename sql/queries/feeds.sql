-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetFeeds :many
SELECT feeds.*, users.name AS user_name
FROM feeds
INNER JOIN users ON feeds.user_id = users.id
ORDER BY feeds.created_at;

-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE url = $1;

-- name: MarkFeedFetched :one
UPDATE feeds
SET last_fetched_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetNextFeedsToFetch :many
-- Healthy feeds (no consecutive failures) are always eligible. A feed with
-- consecutive failures backs off exponentially (2, 4, 8, ... minutes, capped
-- at 60) since its last fetch attempt before it's tried again. Returns up to
-- $1 feeds so callers can fetch a batch concurrently.
SELECT * FROM feeds
WHERE consecutive_failures = 0
   OR last_fetched_at IS NULL
   OR last_fetched_at <= NOW() - (INTERVAL '1 minute' * LEAST(POWER(2, consecutive_failures), 60))
ORDER BY last_fetched_at NULLS FIRST
LIMIT $1;

-- name: MarkFeedFetchSuccess :one
UPDATE feeds
SET consecutive_failures = 0, last_fetch_error = NULL, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkFeedFetchFailure :one
UPDATE feeds
SET consecutive_failures = consecutive_failures + 1, last_fetch_error = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
