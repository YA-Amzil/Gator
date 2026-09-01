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
- **CLI** (`internal/cli`) — command registry, middleware, and handlers (`addfeed`, `follow`, `following`, `agg`, etc.)
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

1. Fetch the next feed to update (the one with the oldest `last_fetched_at`, nulls first)
2. Mark it as fetched
3. Parse its RSS XML
4. Print each post's title and save new posts to the database
5. Wait for the next tick of a `time.Ticker` set to the given duration, then repeat

Example:

```bash
gator agg 1m
```

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
gator agg 30s
gator browse 10
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
| `browse [limit]` | yes | Show saved posts from followed feeds (default limit: 2) |
| `agg <duration>` | – | Continuously scrape feeds (e.g. `agg 1m`, `agg 30s`) |

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

## License

MIT — see [LICENSE](LICENSE).
