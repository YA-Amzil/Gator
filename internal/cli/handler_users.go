package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gator/internal/database"
	"gator/internal/state"
)

func HandlerRegister(s *State, cmd Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: register <name>")
	}
	name := cmd.Args[0]

	ctx := context.Background()
	if _, err := s.DB.GetUserByName(ctx, name); err == nil {
		return fmt.Errorf("user %q already exists", name)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("checking for existing user: %w", err)
	}

	now := time.Now().UTC()
	user, err := s.DB.CreateUser(ctx, database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
	})
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	if err := state.Write(state.Session{CurrentUserName: user.Name}); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	fmt.Printf("User %q created successfully\n", user.Name)
	return nil
}

func HandlerLogin(s *State, cmd Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: login <name>")
	}
	name := cmd.Args[0]

	ctx := context.Background()
	user, err := s.DB.GetUserByName(ctx, name)
	if err != nil {
		return fmt.Errorf("user %q does not exist", name)
	}

	if err := state.Write(state.Session{CurrentUserName: user.Name}); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	fmt.Printf("Logged in as %q\n", user.Name)
	return nil
}

func HandlerReset(s *State, cmd Command) error {
	ctx := context.Background()
	if err := s.DB.DeleteAllUsers(ctx); err != nil {
		return fmt.Errorf("resetting users: %w", err)
	}
	fmt.Println("Database reset: all users (and their feeds, follows, and posts) were deleted")
	return nil
}

func HandlerUsers(s *State, cmd Command) error {
	ctx := context.Background()
	users, err := s.DB.GetUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	sess, err := s.Session()
	if err != nil {
		return fmt.Errorf("reading session: %w", err)
	}

	for _, u := range users {
		if u.Name == sess.CurrentUserName {
			fmt.Printf("* %s (current)\n", u.Name)
		} else {
			fmt.Printf("* %s\n", u.Name)
		}
	}
	return nil
}
