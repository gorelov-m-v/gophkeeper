// Package service provides business logic for the GophKeeper server.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/user/gophkeeper/internal/server/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidEmail indicates the email address is missing or malformed.
	ErrInvalidEmail = errors.New("email must be non-empty and contain '@'")
	// ErrWeakPassword indicates the password does not meet minimum length.
	ErrWeakPassword = errors.New("password must be at least 8 characters")
	// ErrEmptyPassword indicates the password field is empty.
	ErrEmptyPassword = errors.New("password must not be empty")
	// ErrPasswordTooLong indicates the password exceeds the 72-byte bcrypt limit.
	ErrPasswordTooLong = errors.New("password must not exceed 72 bytes")
)

// UserRepository describes user persistence behavior required by AuthService.
type UserRepository interface {
	Create(ctx context.Context, email, passwordHash string) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*repository.UserRow, error)
	GetByEmail(ctx context.Context, email string) (*repository.UserRow, error)
}

// TokenManager describes token behavior required by AuthService and interceptors.
type TokenManager interface {
	GenerateAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	ValidateAccessToken(tokenStr string) (uuid.UUID, error)
	ValidateRefreshToken(tokenStr string) (uuid.UUID, error)
}

// AuthTokens represents a successfully authenticated user session.
type AuthTokens struct {
	UserID       uuid.UUID
	AccessToken  string
	RefreshToken string
}

// AuthService handles user registration, login, and token refresh.
type AuthService struct {
	userRepo     UserRepository
	tokenManager TokenManager
}

// NewAuthService creates a new AuthService with the given dependencies.
func NewAuthService(userRepo UserRepository, tokenManager TokenManager) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		tokenManager: tokenManager,
	}
}

// Register creates a new user account and returns a JWT token pair.
func (s *AuthService) Register(ctx context.Context, email, password string) (*AuthTokens, error) {
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}
	if len(password) > 72 {
		return nil, ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	userID, err := s.userRepo.Create(ctx, email, string(hash))
	if err != nil {
		return nil, err
	}

	return s.issueTokens(userID)
}

// Login authenticates a user by email and password, returning a JWT token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthTokens, error) {
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}
	if password == "" {
		return nil, ErrEmptyPassword
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return s.issueTokens(user.ID)
}

// RefreshToken validates a refresh token and returns a new JWT token pair.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	userID, err := s.tokenManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	return s.issueTokens(userID)
}

func (s *AuthService) issueTokens(userID uuid.UUID) (*AuthTokens, error) {
	accessToken, err := s.tokenManager.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.tokenManager.GenerateRefreshToken(userID)
	if err != nil {
		return nil, err
	}

	return &AuthTokens{
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
