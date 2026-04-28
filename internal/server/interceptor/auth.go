// Package interceptor provides gRPC middleware for the GophKeeper server.
package interceptor

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/user/gophkeeper/internal/server/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

var unauthenticatedMethods = map[string]struct{}{
	"/gophkeeper.auth.v1.AuthService/Register":     {},
	"/gophkeeper.auth.v1.AuthService/Login":        {},
	"/gophkeeper.auth.v1.AuthService/RefreshToken": {},
}

// AuthUnaryInterceptor returns a gRPC unary server interceptor that validates
// access tokens and injects the user ID into the request context.
func AuthUnaryInterceptor(tokenManager service.TokenManager, logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if _, ok := unauthenticatedMethods[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.Warn("missing incoming metadata", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			logger.Warn("missing authorization header", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		tokenStr := strings.TrimPrefix(values[0], "Bearer ")
		userID, err := tokenManager.ValidateAccessToken(tokenStr)
		if err != nil {
			logger.Warn("invalid access token", zap.String("method", info.FullMethod), zap.Error(err))
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		ctx = context.WithValue(ctx, userIDKey, userID)
		return handler(ctx, req)
	}
}

// UserIDFromContext extracts the user ID from the context set by the auth interceptor.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// ContextWithUserID returns a new context with the given user ID set.
func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
