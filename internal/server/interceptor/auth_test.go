package interceptor

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/server/jwt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthInterceptor_SkipsAuthMethods(t *testing.T) {
	tm := jwt.NewTokenManager("secret", 15*time.Minute, 24*time.Hour)
	interceptor := AuthUnaryInterceptor(tm, zap.NewNop())

	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.auth.v1.AuthService/Register"}
	resp, err := interceptor(context.Background(), nil, info, handler)

	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	tm := jwt.NewTokenManager("secret", 15*time.Minute, 24*time.Hour)
	interceptor := AuthUnaryInterceptor(tm, zap.NewNop())
	userID := uuid.New()

	token, err := tm.GenerateAccessToken(userID)
	require.NoError(t, err)

	md := metadata.New(map[string]string{"authorization": "Bearer " + token})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var ctxFromHandler context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		ctxFromHandler = ctx
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.SecretsService/Create"}
	resp, err := interceptor(ctx, nil, info, handler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	gotID, ok := UserIDFromContext(ctxFromHandler)
	assert.True(t, ok)
	assert.Equal(t, userID, gotID)
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	tm := jwt.NewTokenManager("secret", 15*time.Minute, 24*time.Hour)
	interceptor := AuthUnaryInterceptor(tm, zap.NewNop())

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.SecretsService/Get"}
	_, err := interceptor(context.Background(), nil, info, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_MissingAuthorizationHeader(t *testing.T) {
	tm := jwt.NewTokenManager("secret", 15*time.Minute, 24*time.Hour)
	interceptor := AuthUnaryInterceptor(tm, zap.NewNop())

	md := metadata.New(map[string]string{"other": "value"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.SecretsService/Get"}
	_, err := interceptor(ctx, nil, info, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	tm := jwt.NewTokenManager("secret", 15*time.Minute, 24*time.Hour)
	interceptor := AuthUnaryInterceptor(tm, zap.NewNop())

	md := metadata.New(map[string]string{"authorization": "Bearer invalid.token.here"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/gophkeeper.SecretsService/Get"}
	_, err := interceptor(ctx, nil, info, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestUserIDFromContext_Missing(t *testing.T) {
	_, ok := UserIDFromContext(context.Background())
	assert.False(t, ok)
}
