// Package grpchandler provides gRPC service implementations for GophKeeper.
package grpchandler

import (
	"context"
	"errors"

	"github.com/user/gophkeeper/internal/server/repository"
	"github.com/user/gophkeeper/internal/server/service"
	"github.com/user/gophkeeper/pkg/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthHandler implements the gRPC AuthService server.
type AuthHandler struct {
	gen.UnimplementedAuthServiceServer
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler with the given auth service.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register creates a new user account and returns JWT tokens.
func (h *AuthHandler) Register(ctx context.Context, req *gen.RegisterRequest) (*gen.RegisterResponse, error) {
	tokens, err := h.authService.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		if errors.Is(err, service.ErrInvalidEmail) || errors.Is(err, service.ErrWeakPassword) || errors.Is(err, service.ErrPasswordTooLong) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to register user")
	}
	return &gen.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		UserId:       tokens.UserID.String(),
	}, nil
}

// Login authenticates a user and returns JWT tokens.
func (h *AuthHandler) Login(ctx context.Context, req *gen.LoginRequest) (*gen.LoginResponse, error) {
	tokens, err := h.authService.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		if errors.Is(err, service.ErrInvalidEmail) || errors.Is(err, service.ErrEmptyPassword) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	return &gen.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		UserId:       tokens.UserID.String(),
	}, nil
}

// RefreshToken exchanges a valid refresh token for a new token pair.
func (h *AuthHandler) RefreshToken(ctx context.Context, req *gen.RefreshTokenRequest) (*gen.RefreshTokenResponse, error) {
	tokens, err := h.authService.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}
	return &gen.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		UserId:       tokens.UserID.String(),
	}, nil
}
