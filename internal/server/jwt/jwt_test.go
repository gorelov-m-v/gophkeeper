package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := tm.GenerateAccessToken(userID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := tm.ValidateAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := tm.GenerateRefreshToken(userID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := tm.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 0, 0)
	userID := uuid.New()

	token, err := tm.GenerateAccessToken(userID)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)
	_, err = tm.ValidateAccessToken(token)
	assert.Error(t, err)
}

func TestWrongTokenType(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	accessToken, err := tm.GenerateAccessToken(userID)
	require.NoError(t, err)

	_, err = tm.ValidateRefreshToken(accessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")

	refreshToken, err := tm.GenerateRefreshToken(userID)
	require.NoError(t, err)

	_, err = tm.ValidateAccessToken(refreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestInvalidSignature(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute, 24*time.Hour)
	userID := uuid.New()

	token, err := tm.GenerateAccessToken(userID)
	require.NoError(t, err)

	tampered := token + "x"
	_, err = tm.ValidateAccessToken(tampered)
	assert.Error(t, err)

	tm2 := NewTokenManager("different-secret", 15*time.Minute, 24*time.Hour)
	_, err = tm2.ValidateAccessToken(token)
	assert.Error(t, err)
}
