package grpchandler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/server/jwt"
	"github.com/user/gophkeeper/internal/server/repository"
	"github.com/user/gophkeeper/internal/server/service"
	"github.com/user/gophkeeper/pkg/gen"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testJWTSecret = "test-secret-key-32bytes!!"

func newAuthHandlerWithMock(t *testing.T) (*AuthHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)

	tm := jwt.NewTokenManager(testJWTSecret, 15*time.Minute, 24*time.Hour)
	userRepo := repository.NewUserRepo(mock)
	authSvc := service.NewAuthService(userRepo, tm)
	handler := NewAuthHandler(authSvc)

	return handler, mock
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func TestRegister_Success(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	expectedID := uuid.New()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(expectedID))

	resp, err := handler.Register(context.Background(), &gen.RegisterRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.NotEmpty(t, resp.GetRefreshToken())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegister_UserExists(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(repository.ErrUserExists)

	_, err := handler.Register(context.Background(), &gen.RegisterRequest{
		Email:    "dup@example.com",
		Password: "secret123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_Success(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()
	hash := hashPassword(t, "secret123")

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", hash, now, now))

	resp, err := handler.Login(context.Background(), &gen.LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.NotEmpty(t, resp.GetRefreshToken())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_InvalidCredentials(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()
	hash := hashPassword(t, "correct-password")

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email").
		WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", hash, now, now))

	_, err := handler.Login(context.Background(), &gen.LoginRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshToken_Success(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	tm := jwt.NewTokenManager(testJWTSecret, 15*time.Minute, 24*time.Hour)
	userID := uuid.New()
	now := time.Now()
	refreshToken, err := tm.GenerateRefreshToken(userID)
	require.NoError(t, err)

	mock.ExpectQuery("SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "email", "password_hash", "created_at", "updated_at"}).
			AddRow(userID, "user@example.com", "hash", now, now))

	resp, err := handler.RefreshToken(context.Background(), &gen.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.NotEmpty(t, resp.GetRefreshToken())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegister_InternalError(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("unexpected db failure"))

	_, err := handler.Register(context.Background(), &gen.RegisterRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegister_WeakPassword(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	_, err := handler.Register(context.Background(), &gen.RegisterRequest{
		Email:    "user@example.com",
		Password: "short",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "password")
}

func TestLogin_InvalidEmail(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	_, err := handler.Login(context.Background(), &gen.LoginRequest{
		Email:    "",
		Password: "secret123",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "email")
}

func TestLogin_EmptyPassword(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	_, err := handler.Login(context.Background(), &gen.LoginRequest{
		Email:    "user@example.com",
		Password: "",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "password")
}

func TestRefreshToken_Invalid(t *testing.T) {
	handler, mock := newAuthHandlerWithMock(t)
	defer mock.Close()

	_, err := handler.RefreshToken(context.Background(), &gen.RefreshTokenRequest{
		RefreshToken: "invalid-token",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
