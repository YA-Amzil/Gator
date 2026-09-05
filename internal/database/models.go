package database

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
}

type Feed struct {
	ID                  uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Name                string
	Url                 string
	UserID              uuid.UUID
	LastFetchedAt       sql.NullTime
	ConsecutiveFailures int32
	LastFetchError      sql.NullString
}

type FeedWithUser struct {
	Feed
	UserName string
}

type FeedFollow struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uuid.UUID
	FeedID    uuid.UUID
}

type FeedFollowRow struct {
	FeedFollow
	FeedName string
	UserName string
}

type Post struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Url         string
	Description sql.NullString
	PublishedAt sql.NullTime
	FeedID      uuid.UUID
}

type PostWithReadStatus struct {
	Post
	IsRead bool
}

type PostRead struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uuid.UUID
	PostID    uuid.UUID
}
