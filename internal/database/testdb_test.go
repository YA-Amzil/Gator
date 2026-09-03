package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// setupTestDB connects to the Postgres instance named by GATOR_DB_URL and
// returns a Queries wired to it, with all tables truncated for a clean
// slate. Tests are skipped (not failed) when no database is reachable, so
// `go test ./...` still passes in environments without Postgres running.
func setupTestDB(t *testing.T) *Queries {
	t.Helper()

	dbURL := os.Getenv("GATOR_DB_URL")
	if dbURL == "" {
		t.Skip("GATOR_DB_URL not set; skipping database integration test")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("database not reachable at %s: %v", dbURL, err)
	}

	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE posts, feed_follows, feeds, users CASCADE`); err != nil {
		db.Close()
		t.Fatalf("truncating tables: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return New(db)
}

// createTestUser is a shared fixture: most feed/follow/post tests need a
// user to hang records off of, and don't care about its specific fields.
func createTestUser(t *testing.T, q *Queries, name string) User {
	t.Helper()

	now := time.Now().UTC()
	u, err := q.CreateUser(context.Background(), CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
	})
	if err != nil {
		t.Fatalf("createTestUser(%q): %v", name, err)
	}
	return u
}
