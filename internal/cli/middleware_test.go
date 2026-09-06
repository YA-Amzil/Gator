package cli

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"gator/internal/database"
)

func TestMiddlewareLoggedIn_NoSessionFails(t *testing.T) {
	s := newTestState(t)

	called := false
	handler := MiddlewareLoggedIn(func(s *State, cmd Command, user database.User) error {
		called = true
		return nil
	})

	if err := handler(s, Command{Name: "addfeed"}); err == nil {
		t.Fatal("expected an error when no user is logged in, got nil")
	}
	if called {
		t.Error("inner handler should not run when no user is logged in")
	}
}

func TestMiddlewareLoggedIn_PassesCurrentUser(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	var gotUser database.User
	handler := MiddlewareLoggedIn(func(s *State, cmd Command, user database.User) error {
		gotUser = user
		return nil
	})

	if err := handler(s, Command{Name: "addfeed"}); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if gotUser.Name != "alice" {
		t.Errorf("user.Name = %q, want %q", gotUser.Name, "alice")
	}
}

// TestMiddlewareLoggedIn_ServesFromCacheAfterDBRowDeleted proves the cache
// is actually being consulted, not just coincidentally always agreeing with
// the database: it deletes the user's row directly (bypassing every normal
// API) after HandlerRegister's write-through Set populates the cache, then
// confirms MiddlewareLoggedIn still resolves the user — which is only
// possible if it served the cached copy instead of re-querying Postgres.
func TestMiddlewareLoggedIn_ServesFromCacheAfterDBRowDeleted(t *testing.T) {
	s := newTestState(t)

	if err := HandlerRegister(s, Command{Args: []string{"alice"}}); err != nil {
		t.Fatalf("HandlerRegister returned error: %v", err)
	}

	rawDB, err := sql.Open("postgres", os.Getenv("GATOR_DB_URL"))
	if err != nil {
		t.Fatalf("opening raw db connection: %v", err)
	}
	defer rawDB.Close()
	if _, err := rawDB.ExecContext(context.Background(), `DELETE FROM users WHERE name = $1`, "alice"); err != nil {
		t.Fatalf("deleting user row directly: %v", err)
	}

	// Sanity check: without the cache, this lookup would now fail.
	if _, err := s.DB.GetUserByName(context.Background(), "alice"); err == nil {
		t.Fatal("expected the direct DB lookup to fail after deleting the row")
	}

	var gotUser database.User
	handler := MiddlewareLoggedIn(func(s *State, cmd Command, user database.User) error {
		gotUser = user
		return nil
	})

	if err := handler(s, Command{Name: "addfeed"}); err != nil {
		t.Fatalf("handler returned error: %v (cache should have served the user despite the deleted row)", err)
	}
	if gotUser.Name != "alice" {
		t.Errorf("user.Name = %q, want %q", gotUser.Name, "alice")
	}
}
