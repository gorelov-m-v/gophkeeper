package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/server/jwt"
	"github.com/user/gophkeeper/internal/server/repository"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(t *testing.T) (*AuthService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	userRepo := repository.NewUserRepo(mock)
	tm := jwt.NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)
	return NewAuthService(userRepo, tm), mock
}

func TestRegister(t *testing.T) {
	svc, mock := newTestAuthService(t)
	defer mock.Close()

	expectedID := uuid.New()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(expectedID))

	tokens, err := svc.Register(context.Background(), "test@example.com", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, expectedID, tokens.UserID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegisterDuplicate(t *testing.T) {
	svc, mock := newTestAuthService(t)
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(repository.ErrUserExists)

	_, err := svc.Register(context.Background(), "dup@example.com", "password123")
	assert.ErrorIs(t, err, repository.ErrUserExists)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin(t *testing.T) {
	svc, mock := newTestAuthService(t)
	defer mock.Close()

	userID := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	now := time.Now()

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", string(hash), now, now))

	tokens, err := svc.Login(context.Background(), "user@example.com", "correct-password")
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, userID, tokens.UserID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLoginWrongPassword(t *testing.T) {
	svc, mock := newTestAuthService(t)
	defer mock.Close()

	userID := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	now := time.Now()

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", string(hash), now, now))

	_, err := svc.Login(context.Background(), "user@example.com", "wrong-password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshToken(t *testing.T) {
	tm := jwt.NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	userRepo := repository.NewUserRepo(mock)
	svc := NewAuthService(userRepo, tm)

	userID := uuid.New()
	now := time.Now()
	refreshToken, err := tm.GenerateRefreshToken(userID)
	require.NoError(t, err)

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", "hash", now, now))

	tokens, err := svc.RefreshToken(context.Background(), refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, userID, tokens.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}
