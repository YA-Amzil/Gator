# Gator

## Overview

This project is a RSS-powered Blog Aggregator built in Go 1.27.
It fetches RSS feeds, stores posts in a PostgreSQL database, and provides a
CLI for registering users, managing feeds, following other users' feeds, and
running a background aggregator loop.

## Requirements

- Go 1.27+
- Docker (for Postgres)
- [goose](https://github.com/pressly/goose) for running migrations:
  `go install github.com/pressly/goose/v3/cmd/goose@latest`

## Project Structure

- **Config** (`internal/config`) — environment-based configuration loader
- **State** (`internal/state`) — persists the logged-in user between CLI runs
- **Database** (`sql/`, `internal/database`) — migrations, queries, and PostgreSQL access
- **RSS** (`internal/rss`) — XML parsing and feed fetching
- **CLI** (`internal/cli`) — command registry, middleware, and handlers (`addfeed`, `follow`, `following`, `agg`, `read`/`unread`, etc.)
- **Docker** (`docker/`) — PostgreSQL container and supporting compose config

```
main.go                  entrypoint: loads config, wires commands, dispatches
docker/
  docker-compose.yml      Postgres service
  .env.example             docker-compose variable defaults
sql/
  schema/                  goose migrations (up/down)
  queries/                 source-of-truth SQL, mirrored by hand in internal/database
internal/
  config/                  env-var based configuration (DB URL)
  state/                   persists the logged-in user between CLI runs
  database/                hand-written sqlc-style query layer (database/sql + lib/pq)
  rss/                     RSS feed fetching + XML parsing
  cli/                     command registry, middleware, and handlers
```

## Features

- Fetch and parse RSS feeds
- Store feeds, posts, and follow relationships
- Many-to-many user/feed following system
- Background aggregator loop using `time.Ticker`
- Clean, versioned SQL migrations
- Fully containerized PostgreSQL setup
- Idiomatic Go code with clear separation of concerns

## Docker Setup

The project includes a `/docker` folder containing:

- `docker-compose.yml`
- PostgreSQL service
- Volume persistence
- Environment variables for DB configuration (`docker/.env.example`)

Start the database:

```bash
docker compose -f docker/docker-compose.yml up -d
```

Defaults to user/password/db `gator`/`gator`/`gator` on port `5432`
(override via `docker/.env`, copied from `docker/.env.example`).

## RSS Fetching

The aggregator fetches RSS feeds using:

- `http.NewRequestWithContext`
- A custom `User-Agent: gator` header
- `io.ReadAll` + `xml.Unmarshal`
- `html.UnescapeString` on channel/item titles and descriptions

Example feeds used for testing:

- https://www.wagslane.dev/index.xml
- https://techcrunch.com/feed/
- https://news.ycombinator.com/rss

## Following System

Users can follow feeds added by other users.
This is implemented using a `feed_follows` join table with:

- Primary key
- `created_at`, `updated_at`
- `user_id` (FK, cascade delete)
- `feed_id` (FK, cascade delete)
- Unique `(user_id, feed_id)` constraint

Commands:

- `follow <url>` — follow an existing feed
- `following` — list feeds you follow
- `unfollow <url>` — stop following a feed
- `addfeed <name> <url>` — creates a feed and automatically follows it

## Aggregation Loop

The `agg` command runs a continuous loop:

1. Fetch up to `<concurrency>` eligible feeds to update (the least-recently-fetched ones, nulls first)
2. Process each one concurrently, one goroutine per feed:
   - Mark it as fetched
   - Parse its RSS XML
   - Print each post's title and save new posts to the database
3. Wait for all feeds in the batch to finish, then wait for the next tick of a `time.Ticker` set to the given duration, and repeat

Example — fetch up to 5 feeds at a time, every minute:

```bash
gator agg 1m 5
```

A slow or failing feed only holds up its own goroutine, not the rest of the
batch, and each feed's output is printed as one block so concurrent feeds'
lines don't interleave.

### Feed health tracking

Each feed tracks `consecutive_failures` and `last_fetch_error`. A feed that
fetches successfully is always immediately eligible again once it's the
least-recently-fetched feed. A feed that keeps failing backs off
exponentially (2, 4, 8... minutes, capped at 60) before it's retried, so one
broken feed doesn't crowd out healthy ones or get hammered every tick. Run
`gator feeds` to see which feeds are currently unhealthy and why.

## Read/Unread Tracking

Each user tracks their own read/unread status per post, independent of other
users following the same feed. This is implemented using a `post_reads` join
table with:

- Primary key
- `created_at`, `updated_at`
- `user_id` (FK, cascade delete)
- `post_id` (FK, cascade delete)
- Unique `(user_id, post_id)` constraint — a row's presence means read; its
  absence means unread, so no column on `posts` itself changes

Commands:

- `read <post-url>` — mark a post as read (a no-op if already read)
- `unread <post-url>` — mark a post as unread
- `browse` — now shows a `[read]`/`[unread]` marker per post and an unread count

## Installation

```bash
go mod download
go build -o bin/gator .
```

## Usage

```bash
gator register alice
gator addfeed "TechCrunch" https://techcrunch.com/feed/
gator follow https://news.ycombinator.com/rss
gator following
gator feeds
gator agg 30s 5
gator browse 10
gator read https://news.ycombinator.com/item?id=1
```

## Commands

| Command | Auth required | Description |
|---|---|---|
| `register <name>` | – | Create a user and log in as them |
| `login <name>` | – | Switch the current user |
| `reset` | – | Delete all users (cascades to their feeds/follows/posts) |
| `users` | – | List all users, marking the current one |
| `addfeed <name> <url>` | yes | Create a feed and auto-follow it |
| `feeds` | – | List all feeds and who added them |
| `follow <url>` | yes | Follow an existing feed by URL |
| `following` | yes | List feeds the current user follows |
| `unfollow <url>` | yes | Stop following a feed |
| `browse [limit]` | yes | Show saved posts from followed feeds, with read/unread status (default limit: 2) |
| `read <post-url>` | yes | Mark a post as read |
| `unread <post-url>` | yes | Mark a post as unread |
| `agg <duration> <concurrency>` | – | Continuously scrape up to `concurrency` feeds at a time (e.g. `agg 1m 5`) |

Commands marked "Auth required" need a logged-in user (`register`/`login`
sets one); the login state is stored in `~/.gator/session.json`, separate
from the env-var-based DB config.

## Setup (step by step)

1. **Start Postgres** — `docker compose -f docker/docker-compose.yml up -d`
2. **Configure the app** — `cp .env.example .env` (sets `GATOR_DB_URL`, loaded automatically at startup)
3. **Run migrations**:
   ```bash
   cd sql/schema
   goose postgres "$GATOR_DB_URL" up
   cd ../..
   ```
   or `make migrate-up`
4. **Build** — `go build -o bin/gator .`

## Database Layer

`internal/database` is hand-written in the style `sqlc` would generate —
each `*.sql.go` file mirrors a query file under `sql/queries/` exactly, using
`database/sql` and `github.com/lib/pq`. There's no code-generation step to
run; the Go and SQL are kept in sync by hand.

## Testing

```bash
go test ./...
```

`internal/rss`, `internal/config`, and `internal/state` are pure unit tests
with no external dependencies. `internal/database` and `internal/cli` run
integration tests against a real Postgres database, read from `GATOR_DB_URL`
(the same variable used at runtime) — they truncate all tables before each
test for a clean slate, and isolate the session file (`internal/state`) in a
temp directory so they never touch your real login session.

If `GATOR_DB_URL` isn't set, or the database isn't reachable, those tests are
skipped rather than failed, so `go test ./...` passes in any environment. To
run the full suite locally:

```bash
docker compose -f docker/docker-compose.yml up -d
export GATOR_DB_URL="postgres://gator:gator@localhost:5432/gator?sslmode=disable"
go test -p 1 ./...
```

(On Windows PowerShell, use `$env:GATOR_DB_URL = "..."` instead of `export`.)

`-p 1` matters: `internal/database` and `internal/cli` both truncate the same
shared tables against one live Postgres instance, and `go test ./...` runs
different packages' tests in parallel by default — without `-p 1` the two
packages race and intermittently wipe each other's fixtures mid-test.

Since `agg` now fetches feeds concurrently, CI also runs with the race
detector (`go test -p 1 -race ./...`). That requires cgo and a C compiler, so
it may not run on every machine (e.g. a Windows box without a C toolchain) —
CI is the reliable place to catch data races.

## Continuous Integration

`.github/workflows/ci.yml` runs on every push and pull request: it spins up a
Postgres service container, builds, vets, applies migrations with goose, and
runs the full test suite with the race detector. See [Branching and workflow](CLAUDE.md#branching-and-workflow)
in `CLAUDE.md` for how feature branches, tests, and CI fit together.

## License

MIT — see [LICENSE](LICENSE).
