# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o bin/gator .          # or: make build

# Run
./bin/gator <command> [args]     # or: make run (builds then runs with no args)

# Vet / static check
go vet ./...

# Postgres (local dev)
docker compose -f docker/docker-compose.yml up -d     # or: make docker-up
docker compose -f docker/docker-compose.yml down       # or: make docker-down

# Migrations (requires: go install github.com/pressly/goose/v3/cmd/goose@latest)
cd sql/schema && goose postgres "$GATOR_DB_URL" up      # or: make migrate-up
cd sql/schema && goose postgres "$GATOR_DB_URL" down     # or: make migrate-down

# Tests (unit tests always run; DB-backed tests need Postgres up + GATOR_DB_URL set,
# and are skipped — not failed — otherwise)
go test ./...
go test ./internal/database/... -run TestGetNextFeedToFetch -v   # single test example
```

`.env` (copied from `.env.example`) sets `GATOR_DB_URL` and is loaded
automatically at startup via godotenv, but `go test` does **not** load
`.env` — export `GATOR_DB_URL` in the shell (or `$env:GATOR_DB_URL` in
PowerShell) before running DB-backed tests.

## Testing conventions

`internal/rss`, `internal/config`, `internal/state` are pure unit tests. `internal/database` and `internal/cli` are integration tests against real Postgres:
- Each package has a `newTestState`/`setupTestDB` helper (in a `testdb_test.go` file) that opens `GATOR_DB_URL`, `t.Skip`s if unset or unreachable, and `TRUNCATE`s `posts, feed_follows, feeds, users CASCADE` before the test body runs — tests assume an empty schema, not that migrations create it.
- `internal/cli` tests additionally redirect the OS home directory (`t.Setenv("USERPROFILE", ...)` / `HOME` on non-Windows) to a temp dir per test, since `internal/state` reads/writes `~/.gator/session.json` directly with no injectable path — never run these against a real `$HOME`.
- Because `internal/cli`'s DB helper can't reach `internal/database`'s unexported `setupTestDB` across the package boundary, it duplicates the same truncate/skip logic — keep both in sync if the schema changes.

## Architecture

Gator is a single Go CLI binary (`main.go`) backed by Postgres. The
`internal/database` package is **hand-written in the style `sqlc` would
generate**, not code-generated — there is no `sqlc generate` step. Every
`sql/queries/*.sql` file has a corresponding hand-written `internal/database/*.sql.go`
file with a matching SQL string constant, `*Params` struct, and method on
`*database.Queries`. When changing a query, edit both files together and keep
them in sync manually.

Similarly, `sql/schema/*.sql` are goose-formatted migrations (`-- +goose Up`
/ `-- +goose Down`) — the Go model structs in `internal/database/models.go`
must have fields in the same column order as `SELECT *` would return, since
scanning is positional (`rows.Scan(&f.ID, &f.CreatedAt, ...)`).

**Command dispatch flow** (`internal/cli`):
- `main.go` builds a `cli.State` (wraps `*database.Queries` + `*config.Config`) and registers each command name to a `cli.Handler` on a `cli.Commands` registry, then dispatches on `os.Args[1]`.
- Commands that require a logged-in user are wrapped in `cli.MiddlewareLoggedIn(...)`, which reads the session (see below), loads the `database.User` by name, and passes it as a third argument to the inner `func(*State, Command, database.User) error` handler. Handlers needing no auth (`register`, `login`, `reset`, `users`, `feeds`, `agg`) are registered directly as `func(*State, Command) error`.
- Handlers are split by concern across `handler_users.go`, `handler_feeds.go`, `handler_follows.go`, `handler_posts.go`, `handler_agg.go` — follow that split when adding new commands.

**Two-tier persistence** — don't conflate these:
- `internal/config`: environment-variable config (`GATOR_DB_URL`), loaded once per process via `config.Load()`.
- `internal/state`: the *currently logged-in user*, persisted between separate CLI invocations as JSON at `~/.gator/session.json`. `register`/`login` write it; `MiddlewareLoggedIn` and `users` read it. This is session data, not config — keep it out of `internal/config`.

**Aggregation loop** (`internal/cli/handler_agg.go`): `agg <duration>` parses
the duration with `time.ParseDuration` and runs `scrapeFeeds` immediately,
then again on every `time.Ticker` tick, forever. Each `scrapeFeeds` call:
picks the feed with the oldest `last_fetched_at` (`GetNextFeedToFetch`,
`ORDER BY last_fetched_at NULLS FIRST` so unfetched feeds go first) → marks
it fetched (`MarkFeedFetched`) → fetches/parses its RSS via
`internal/rss.FetchFeed` → saves each item as a post. `CreatePost` uses
`ON CONFLICT (url) DO NOTHING RETURNING *`, so a duplicate URL returns
`sql.ErrNoRows`, which `savePost` treats as an expected no-op, not an error.

**RSS fetching** (`internal/rss/rss.go`): `FetchFeed` sets
`User-Agent: gator`, and HTML-unescapes exactly four fields after parsing —
channel title/description and each item's title/description. Preserve that
exact behavior (fields, header, unescape targets) if touching this file, since
it mirrors a spec other tooling depends on.

**Schema relationships**: `users` 1—N `feeds` (creator, cascade delete) ←M—N→
`feed_follows` joins `users` and `feeds` (unique `(user_id, feed_id)`,
cascade both directions) — `feeds` 1—N `posts` (cascade delete, unique
`posts.url`).

## Commit conventions

Conventional Commits only, lowercase types: `chore`, `feat`, `fix`,
`refactor`, `docs`, `test`. Include a scope when relevant, e.g.
`feat(cli)`, `fix(rss)`, `refactor(db)`, `chore(docker)`. Commit
step-by-step — one logical change per commit, never bundle unrelated
changes. Every commit from 2026-09-01 onward includes a trailing
`Co-authored-by: Claude <noreply@anthropic.com>` line (the initial history
before that date does not).
