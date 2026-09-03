-- +goose Up
ALTER TABLE feeds
    ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_fetch_error TEXT;

-- +goose Down
ALTER TABLE feeds
    DROP COLUMN consecutive_failures,
    DROP COLUMN last_fetch_error;
