package cli

import (
	"context"
	"testing"

	"gator/internal/database"
)

func TestHandlerRegister_CreatesUserAndLogsIn(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Name: "register", Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	if _, err := s.DB.GetUserByName(context.Background(), "alice"); err != nil {
		t.Errorf("expected user %q to exist in the database: %v", "alice", err)
	}

	sess, err := s.Session()
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if sess.CurrentUserName != "alice" {
		t.Errorf("CurrentUserName = %q, want %q (register should log the new user in)", sess.CurrentUserName, "alice")
	}
}

func TestHandlerRegister_DuplicateNameFails(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("first HandlerRegister returned error: %v", err)
	}
	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err == nil {
		t.Fatal("expected an error registering a duplicate name, got nil")
	}
}

func TestHandlerRegister_WrongArgCount(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{}}); err == nil {
		t.Fatal("expected an error with no name argument, got nil")
	}
}

func TestHandlerLogin_SwitchesCurrentUser(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}
	if err := HandlerRegister(s, Command{Args: []string{"bob"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	if err := HandlerLogin(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerLogin returned error: %v", err)
	}

	sess, err := s.Session()
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if sess.CurrentUserName != "alice" {
		t.Errorf("CurrentUserName = %q, want %q", sess.CurrentUserName, "alice")
	}
}

func TestHandlerLogin_UnknownUserFails(t *testing.T) {
	s := newTestState(t)

	if err := HandlerLogin(s, Command{Args: []string{"nobody"}}); err == nil {
		t.Fatal("expected an error logging in as a nonexistent user, got nil")
	}
}

func TestHandlerReset_DeletesAllUsers(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	if err := HandlerReset(s, Command{}); err != nil {
		t.Fatalf("HandlerReset returned error: %v", err)
	}

	users, err := s.DB.GetUsers(context.Background())
	if err != nil {
		t.Fatalf("GetUsers returned error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("len(users) = %d, want 0 after reset", len(users))
	}
}

// TestHandlerReset_InvalidatesUserCache guards against a lingering session
// being silently served a stale cached identity for a user that reset just
// deleted. Without HandlerReset invalidating the cache, MiddlewareLoggedIn
// would keep resolving "alice" from the pre-reset cache entry and only fail
// confusingly later (e.g. a foreign-key violation on whatever the stale
// user ID gets used for) instead of failing clearly, here, with the correct
// "no such user" error.
func TestHandlerReset_InvalidatesUserCache(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	if err := HandlerReset(s, Command{}); err != nil {
		t.Fatalf("HandlerReset returned error: %v", err)
	}

	// The session file (from register) still says "alice", but the user no
	// longer exists — reset does not log anyone out. Without cache
	// invalidation, this would incorrectly succeed with the stale cached user.
	handler := MiddlewareLoggedIn(func(s *State, cmd Command, user database.User) error {
		return nil
	})
	if err := handler(s, Command{Name: "addfeed"}); err == nil {
		t.Fatal("expected an error resolving a deleted user after reset, got nil (stale cache entry was served)")
	}
}

func TestHandlerUsers_DoesNotError(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	if err := HandlerUsers(s, Command{}); err != nil {
		t.Fatalf("HandlerUsers returned error: %v", err)
	}
}
