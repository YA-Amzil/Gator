package database

import "database/sql"

// Queries wraps a *sql.DB and exposes hand-written, sqlc-style query methods
// generated from the SQL statements in sql/queries.
type Queries struct {
	db *sql.DB
}

func New(db *sql.DB) *Queries {
	return &Queries{db: db}
}
