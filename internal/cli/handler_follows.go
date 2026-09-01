package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

func HandlerFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: follow <url>")
	}
	url := cmd.Args[0]

	ctx := context.Background()
	feed, err := s.DB.GetFeedByURL(ctx, url)
	if err != nil {
		return fmt.Errorf("no feed found for url %q: %w", url, err)
	}

	now := time.Now().UTC()
	follow, err := s.DB.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("following feed: %w", err)
	}

	fmt.Printf("%s is now following %s\n", follow.UserName, follow.FeedName)
	return nil
}

func HandlerFollowing(s *State, cmd Command, user database.User) error {
	ctx := context.Background()
	follows, err := s.DB.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("fetching follows: %w", err)
	}

	if len(follows) == 0 {
		fmt.Println("You are not following any feeds yet")
		return nil
	}

	fmt.Printf("Feeds followed by %s:\n", user.Name)
	for _, f := range follows {
		fmt.Printf("* %s\n", f.FeedName)
	}
	return nil
}

func HandlerUnfollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: unfollow <url>")
	}
	url := cmd.Args[0]

	ctx := context.Background()
	feed, err := s.DB.GetFeedByURL(ctx, url)
	if err != nil {
		return fmt.Errorf("no feed found for url %q: %w", url, err)
	}

	if err := s.DB.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}); err != nil {
		return fmt.Errorf("unfollowing feed: %w", err)
	}

	fmt.Printf("%s unfollowed %s\n", user.Name, feed.Name)
	return nil
}
