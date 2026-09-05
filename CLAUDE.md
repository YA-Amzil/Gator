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
# and are skipped — not failed — otherwise). -p 1 is required for the full suite:
# see the note below on why.
go test -p 1 ./...
go test ./internal/database/... -run TestGetNextFeedsToFetch -v   # single test example (single package is always safe without -p 1)

# Race detector (requires cgo + a C compiler, e.g. gcc; CI runs this, a Windows box without
# a C toolchain cannot)
go test -p 1 -race ./...
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
- `internal/database` and `internal/cli` both truncate the same live tables against one shared Postgres instance, and `go test` runs different packages' binaries in parallel by default — always run the full suite with `go test -p 1 ./...` (as CI does), or the two packages race and truncate each other's fixtures mid-test. A single package (`go test ./internal/database/...`) is safe without `-p 1` since its own tests still run sequentially within one binary.

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

**Aggregation loop** (`internal/cli/handler_agg.go`): `agg <duration> <concurrency>`
parses the duration with `time.ParseDuration` and runs `scrapeFeeds`
immediately, then again on every `time.Ticker` tick, forever. Each
`scrapeFeeds` call fetches up to `concurrency` eligible feeds in one query
(`GetNextFeedsToFetch`, `ORDER BY last_fetched_at NULLS FIRST` so unfetched
feeds go first) and processes each in its own goroutine via `scrapeFeed`,
joined with a `sync.WaitGroup` — a slow or failing feed can't block the
others. `scrapeFeed` marks the feed fetched (`MarkFeedFetched`, claiming it
up front before the HTTP call) → fetches/parses its RSS via
`internal/rss.FetchFeed`, wrapped in `context.WithTimeout(ctx, fetchTimeout)`
(30s, a package `var` so tests can shrink it) so a server that hangs
mid-response can't block its goroutine — and therefore the whole batch's
`wg.Wait()` — forever → saves each item as a post. `CreatePost` uses
`ON CONFLICT (url) DO NOTHING RETURNING *`, so a duplicate URL returns
`sql.ErrNoRows`, which `savePost` treats as an expected no-op, not an error.
Each goroutine buffers its own output (`strings.Builder`) and prints it in a
single `fmt.Print` call so concurrent feeds' lines don't interleave.

**Feed health tracking** (`feeds.consecutive_failures`, `feeds.last_fetch_error`):
a fetch attempt calls `MarkFeedFetched` first to claim the feed (sets
`last_fetched_at`, independent of outcome — this is also what lets
concurrent goroutines avoid double-claiming a feed, since each claim and its
fetch happen sequentially within one `scrapeFeed` call), then either
`MarkFeedFetchSuccess` (resets the failure streak) or `MarkFeedFetchFailure`
(increments it and stores the error) once the fetch attempt resolves.
`GetNextFeedsToFetch` always considers healthy feeds (`consecutive_failures = 0`)
immediately eligible, but a feed with failures backs off exponentially — 2,
4, 8... minutes, capped at 60 — since its `last_fetched_at`, computed
entirely in SQL (`LEAST(POWER(2, consecutive_failures), 60)` minutes). The
`feeds` CLI command surfaces unhealthy feeds (failure count + last error).

**RSS fetching** (`internal/rss/rss.go`): `FetchFeed` sets
`User-Agent: gator`, and HTML-unescapes exactly four fields after parsing —
channel title/description and each item's title/description. Preserve that
exact behavior (fields, header, unescape targets) if touching this file, since
it mirrors a spec other tooling depends on.

**Read/unread tracking** (`post_reads` table): a post is "read" for a user
if a `(user_id, post_id)` row exists in `post_reads` — absence means unread,
so no column on `posts` itself needs updating and multiple users can track
read state on the same shared post independently. `read <post-url>` /
`unread <post-url>` reference posts by URL (`GetPostByURL`), the same
convention `follow`/`unfollow` use for feeds. `MarkPostRead` uses
`ON CONFLICT (user_id, post_id) DO NOTHING RETURNING *`, so re-marking an
already-read post returns `sql.ErrNoRows`, treated as a no-op by
`HandlerRead` (mirrors the `CreatePost` dedup pattern). `GetPostsForUser`
LEFT JOINs `post_reads` to expose `PostWithReadStatus.IsRead`; `browse`
prints an unread count via `CountUnreadPostsForUser` alongside each post's
`[read]`/`[unread]` marker.

**Schema relationships**: `users` 1—N `feeds` (creator, cascade delete) ←M—N→
`feed_follows` joins `users` and `feeds` (unique `(user_id, feed_id)`,
cascade both directions) — `feeds` 1—N `posts` (cascade delete, unique
`posts.url`) ←M—N→ `post_reads` joins `users` and `posts` (unique
`(user_id, post_id)`, cascade both directions).

## Branching and workflow

- Before starting any new feature, create a dedicated branch off `main` named after the feature (e.g. `feed-health-tracking`) — never commit feature work directly to `main`.
- When the feature is complete, run the full test suite (`go test ./...`, with Postgres running and `GATOR_DB_URL` set) and confirm it's green before merging.
- Merge by rebasing the feature branch onto `main`, then fast-forwarding `main` to it — never create a merge commit:
  ```bash
  git checkout feed-health-tracking
  git rebase main
  go test ./...                     # re-verify after rebase
  git checkout main
  git merge --ff-only feed-health-tracking
  git branch -d feed-health-tracking
  ```
  `main` must stay linear; if `git merge --ff-only` refuses (main moved since the branch was cut), rebase again rather than falling back to a merge commit.
- CI (`.github/workflows/ci.yml`) runs build, vet, and the full test suite against a real Postgres service container on every push and pull request — a feature isn't done until CI is green, in addition to local tests passing.

## Commit conventions

Conventional Commits only, lowercase types: `chore`, `feat`, `fix`,
`refactor`, `docs`, `test`. Include a scope when relevant, e.g.
`feat(cli)`, `fix(rss)`, `refactor(db)`, `chore(docker)`. Commit
step-by-step — one logical change per commit, never bundle unrelated
changes. Every commit from 2026-09-01 onward includes a trailing
`Co-authored-by: Claude <noreply@anthropic.com>` line (the initial history
before that date does not).
