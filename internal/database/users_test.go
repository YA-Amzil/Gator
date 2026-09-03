package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateAndGetUser(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	created, err := q.CreateUser(ctx, CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      "alice",
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if created.Name != "alice" {
		t.Errorf("Name = %q, want %q", created.Name, "alice")
	}

	byName, err := q.GetUserByName(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByName returned error: %v", err)
	}
	if byName.ID != created.ID {
		t.Errorf("GetUserByName ID = %v, want %v", byName.ID, created.ID)
	}

	byID, err := q.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if byID.Name != "alice" {
		t.Errorf("GetUserByID Name = %q, want %q", byID.Name, "alice")
	}
}

func TestCreateUser_DuplicateNameFails(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	params := CreateUserParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "alice"}
	if _, err := q.CreateUser(ctx, params); err != nil {
		t.Fatalf("first CreateUser returned error: %v", err)
	}

	params.ID = uuid.New()
	if _, err := q.CreateUser(ctx, params); err == nil {
		t.Fatal("expected an error creating a second user with the same name, got nil")
	}
}

func TestGetUserByName_NotFound(t *testing.T) {
	q := setupTestDB(t)

	_, err := q.GetUserByName(context.Background(), "nobody")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestGetUsers_OrderedByName(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, name := range []string{"charlie", "alice", "bob"} {
		if _, err := q.CreateUser(ctx, CreateUserParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: name}); err != nil {
			t.Fatalf("CreateUser(%q) returned error: %v", name, err)
		}
	}

	users, err := q.GetUsers(ctx)
	if err != nil {
		t.Fatalf("GetUsers returned error: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(users))
	}

	want := []string{"alice", "bob", "charlie"}
	for i, u := range users {
		if u.Name != want[i] {
			t.Errorf("users[%d].Name = %q, want %q", i, u.Name, want[i])
		}
	}
}

func TestDeleteAllUsers(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := q.CreateUser(ctx, CreateUserParams{ID: uuid.New(), CreatedAt: now, UpdatedAt: now, Name: "alice"}); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if err := q.DeleteAllUsers(ctx); err != nil {
		t.Fatalf("DeleteAllUsers returned error: %v", err)
	}

	users, err := q.GetUsers(ctx)
	if err != nil {
		t.Fatalf("GetUsers returned error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("len(users) = %d, want 0 after DeleteAllUsers", len(users))
	}
}
