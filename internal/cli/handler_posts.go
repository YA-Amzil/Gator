package cli

import (
	"context"
	"fmt"
	"strconv"

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
		fmt.Println("No posts found yet. Follow some feeds and run 'agg <duration>' first.")
		return nil
	}

	for _, p := range posts {
		fmt.Printf("* %s\n  %s\n", p.Title, p.Url)
	}
	return nil
}
