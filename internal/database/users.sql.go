package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createUser = `INSERT INTO users (id, created_at, updated_at, name)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, updated_at, name`

type CreateUserParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRowContext(ctx, createUser, arg.ID, arg.CreatedAt, arg.UpdatedAt, arg.Name)
	var u User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.Name)
	return u, err
}

const getUserByName = `SELECT id, created_at, updated_at, name FROM users WHERE name = $1`

func (q *Queries) GetUserByName(ctx context.Context, name string) (User, error) {
	row := q.db.QueryRowContext(ctx, getUserByName, name)
	var u User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.Name)
	return u, err
}

const getUserByID = `SELECT id, created_at, updated_at, name FROM users WHERE id = $1`

func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	row := q.db.QueryRowContext(ctx, getUserByID, id)
	var u User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.Name)
	return u, err
}

const getUsers = `SELECT id, created_at, updated_at, name FROM users ORDER BY name`

func (q *Queries) GetUsers(ctx context.Context) ([]User, error) {
	rows, err := q.db.QueryContext(ctx, getUsers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.Name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

const deleteAllUsers = `DELETE FROM users`

func (q *Queries) DeleteAllUsers(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, deleteAllUsers)
	return err
}
