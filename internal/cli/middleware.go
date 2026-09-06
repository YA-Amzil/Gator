package cli

import (
	"context"
	"fmt"

	"gator/internal/database"
)

type loggedInHandler func(s *State, cmd Command, user database.User) error

// MiddlewareLoggedIn wraps a handler that requires a logged-in user,
// resolving the current session's user before delegating. The user lookup
// is the single hottest read in the CLI — it runs on every authenticated
// command — so it's served from the cache when possible, falling back to
// the database on a miss (or if caching is disabled/unavailable).
func MiddlewareLoggedIn(handler loggedInHandler) Handler {
	return func(s *State, cmd Command) error {
		sess, err := s.Session()
		if err != nil {
			return fmt.Errorf("reading session: %w", err)
		}
		if sess.CurrentUserName == "" {
			return fmt.Errorf("you must be logged in to use '%s'; run 'login <name>' first", cmd.Name)
		}

		ctx := context.Background()

		if user, ok := s.Users.Get(ctx, sess.CurrentUserName); ok {
			return handler(s, cmd, user)
		}

		user, err := s.DB.GetUserByName(ctx, sess.CurrentUserName)
		if err != nil {
			return fmt.Errorf("could not find logged-in user %q: %w", sess.CurrentUserName, err)
		}
		s.Users.Set(ctx, user)

		return handler(s, cmd, user)
	}
}
