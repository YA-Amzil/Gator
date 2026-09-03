package cli

import (
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
