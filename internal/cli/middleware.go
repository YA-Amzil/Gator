package cli

import (
	"context"
	"fmt"

	"gator/internal/database"
)

type loggedInHandler func(s *State, cmd Command, user database.User) error

// MiddlewareLoggedIn wraps a handler that requires a logged-in user,
// resolving the current session's user from the database before delegating.
func MiddlewareLoggedIn(handler loggedInHandler) Handler {
	return func(s *State, cmd Command) error {
		sess, err := s.Session()
		if err != nil {
			return fmt.Errorf("reading session: %w", err)
		}
		if sess.CurrentUserName == "" {
			return fmt.Errorf("you must be logged in to use '%s'; run 'login <name>' first", cmd.Name)
		}

		user, err := s.DB.GetUserByName(context.Background(), sess.CurrentUserName)
		if err != nil {
			return fmt.Errorf("could not find logged-in user %q: %w", sess.CurrentUserName, err)
		}

		return handler(s, cmd, user)
	}
}
