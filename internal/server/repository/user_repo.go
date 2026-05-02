// Package repository provides database access for GophKeeper domain entities.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/user/gophkeeper/internal/retry"
)

// ErrUserExists is returned when a user with the given email already exists.
var ErrUserExists = errors.New("user already exists")

// ErrUserNotFound is returned when no user matches the given criteria.
var ErrUserNotFound = errors.New("user not found")

// UserRow represents a row from the users table.
type UserRow struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DBTX is an interface satisfied by *pgxpool.Pool, pgx.Conn, and pgx.Tx,
// allowing repositories to work with any of these connection types.
type DBTX interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// UserRepo provides database operations for users.
type UserRepo struct {
	pool DBTX
}

// NewUserRepo creates a new UserRepo backed by the given connection pool.
func NewUserRepo(pool DBTX) *UserRepo {
	return &UserRepo{pool: pool}
}

// Create inserts a new user and returns the generated ID.
func (r *UserRepo) Create(ctx context.Context, email, passwordHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
			email, passwordHash,
		).Scan(&id)
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, ErrUserExists
		}
		return uuid.Nil, err
	}
	return id, nil
}

// GetByID retrieves a user by their unique ID.
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*UserRow, error) {
	var user UserRow
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1",
			id,
		).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email address.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*UserRow, error) {
	var user UserRow
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = $1",
			email,
		).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}
