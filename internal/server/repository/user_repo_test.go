package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewUserRepo(mock)
	expectedID := uuid.New()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("test@example.com", "hashpwd").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(expectedID))

	id, err := repo.Create(context.Background(), "test@example.com", "hashpwd")
	require.NoError(t, err)
	assert.Equal(t, expectedID, id)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUserDuplicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewUserRepo(mock)

	pgErr := &pgconn.PgError{Code: "23505"}
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("dup@example.com", "hashpwd").
		WillReturnError(pgErr)

	_, err = repo.Create(context.Background(), "dup@example.com", "hashpwd")
	assert.ErrorIs(t, err, ErrUserExists)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewUserRepo(mock)
	expectedID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("test@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(expectedID, "test@example.com", "hashpwd", now, now))

	user, err := repo.GetByEmail(context.Background(), "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, expectedID, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hashpwd", user.PasswordHash)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByEmailNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewUserRepo(mock)

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("missing@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}))

	_, err = repo.GetByEmail(context.Background(), "missing@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)

	require.NoError(t, mock.ExpectationsWereMet())
}
