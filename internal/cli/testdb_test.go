package cli

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"gator/internal/config"
	"gator/internal/database"
)

// newTestState connects to the Postgres instance named by GATOR_DB_URL,
// truncates all tables for a clean slate, and points the OS "home
// directory" lookup at a temp dir so session.json never touches the real
// user's session. Tests are skipped (not failed) when no database is
// reachable, so `go test ./...` still passes without Postgres running.
func newTestState(t *testing.T) *State {
	t.Helper()

	dbURL := os.Getenv("GATOR_DB_URL")
	if dbURL == "" {
		t.Skip("GATOR_DB_URL not set; skipping cli integration test")
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

	tempHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tempHome)
	} else {
		t.Setenv("HOME", tempHome)
	}

	return &State{
		DB:  database.New(db),
		Cfg: &config.Config{DBURL: dbURL},
	}
}
