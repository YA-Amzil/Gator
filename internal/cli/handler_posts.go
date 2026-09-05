package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

const defaultBrowseLimit = 2

func HandlerBrowse(s *State, cmd Command, user database.User) error {
	limit := int32(defaultBrowseLimit)
	if len(cmd.Args) == 1 {
		parsed, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("invalid limit %q: %w", cmd.Args[0], err)
		}
		limit = int32(parsed)
	} else if len(cmd.Args) > 1 {
		return fmt.Errorf("usage: browse [limit]")
	}

	ctx := context.Background()
	posts, err := s.DB.GetPostsForUser(ctx, database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  limit,
	})
	if err != nil {
		return fmt.Errorf("fetching posts: %w", err)
	}

	if len(posts) == 0 {
		fmt.Println("No posts found yet. Follow some feeds and run 'agg <duration> <concurrency>' first.")
		return nil
	}

	unread, err := s.DB.CountUnreadPostsForUser(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("counting unread posts: %w", err)
	}
	fmt.Printf("Showing %d posts (%d unread)\n", len(posts), unread)

	for _, p := range posts {
		status := "read"
		if !p.IsRead {
			status = "unread"
		}
		fmt.Printf("* [%s] %s\n  %s\n", status, p.Title, p.Url)
	}
	return nil
}

func HandlerRead(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: read <post-url>")
	}
	url := cmd.Args[0]

	ctx := context.Background()
	post, err := s.DB.GetPostByURL(ctx, url)
	if err != nil {
		return fmt.Errorf("no post found for url %q: %w", url, err)
	}

	now := time.Now().UTC()
	_, err = s.DB.MarkPostRead(ctx, database.MarkPostReadParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		PostID:    post.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("%q was already marked read\n", post.Title)
			return nil
		}
		return fmt.Errorf("marking post read: %w", err)
	}

	fmt.Printf("Marked %q as read\n", post.Title)
	return nil
}

func HandlerUnread(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: unread <post-url>")
	}
	url := cmd.Args[0]

	ctx := context.Background()
	post, err := s.DB.GetPostByURL(ctx, url)
	if err != nil {
		return fmt.Errorf("no post found for url %q: %w", url, err)
	}

	if err := s.DB.MarkPostUnread(ctx, database.MarkPostUnreadParams{UserID: user.ID, PostID: post.ID}); err != nil {
		return fmt.Errorf("marking post unread: %w", err)
	}

	fmt.Printf("Marked %q as unread\n", post.Title)
	return nil
}
