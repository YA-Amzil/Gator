package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
)

func HandlerAddFeed(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}
	name, url := cmd.Args[0], cmd.Args[1]

	ctx := context.Background()
	now := time.Now().UTC()

	feed, err := s.DB.CreateFeed(ctx, database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
		Url:       url,
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("creating feed: %w", err)
	}

	follow, err := s.DB.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("following newly created feed: %w", err)
	}

	fmt.Printf("Feed created: %s (%s)\n", feed.Name, feed.Url)
	fmt.Printf("%s is now following %s\n", follow.UserName, follow.FeedName)
	return nil
}

func HandlerFeeds(s *State, cmd Command) error {
	ctx := context.Background()
	feeds, err := s.DB.GetFeeds(ctx)
	if err != nil {
		return fmt.Errorf("fetching feeds: %w", err)
	}

	if len(feeds) == 0 {
		fmt.Println("No feeds yet. Add one with 'addfeed <name> <url>'.")
		return nil
	}

	for _, f := range feeds {
		fmt.Printf("* %s (%s) - added by %s\n", f.Name, f.Url, f.UserName)
	}
	return nil
}
