// Package jwt provides JWT token generation and validation for GophKeeper.
package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenManager handles creation and validation of JWT access and refresh tokens.
type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenManager creates a new TokenManager with the given secret and TTL durations.
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GenerateAccessToken creates a signed HS256 access token for the given user ID.
func (tm *TokenManager) GenerateAccessToken(userID uuid.UUID) (string, error) {
	return tm.generateToken(userID, "access", tm.accessTTL)
}

// GenerateRefreshToken creates a signed HS256 refresh token for the given user ID.
func (tm *TokenManager) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	return tm.generateToken(userID, "refresh", tm.refreshTTL)
}

// ValidateAccessToken parses and validates an access token, returning the user ID.
func (tm *TokenManager) ValidateAccessToken(tokenStr string) (uuid.UUID, error) {
	return tm.validateToken(tokenStr, "access")
}

// ValidateRefreshToken parses and validates a refresh token, returning the user ID.
func (tm *TokenManager) ValidateRefreshToken(tokenStr string) (uuid.UUID, error) {
	return tm.validateToken(tokenStr, "refresh")
}

func (tm *TokenManager) generateToken(userID uuid.UUID, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwtv5.MapClaims{
		"sub":  userID.String(),
		"jti":  uuid.New().String(),
		"exp":  jwtv5.NewNumericDate(now.Add(ttl)),
		"iat":  jwtv5.NewNumericDate(now),
		"type": tokenType,
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

func (tm *TokenManager) validateToken(tokenStr, expectedType string) (uuid.UUID, error) {
	token, err := jwtv5.Parse(tokenStr, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("invalid token claims")
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != expectedType {
		return uuid.Nil, errors.New("invalid token type")
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, errors.New("invalid user ID in token")
	}

	return userID, nil
}
