package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewPostgresPool_InvalidDSN(t *testing.T) {
	_, err := NewPostgresPool(context.Background(), "postgres://invalid:invalid@localhost:1/nonexistent?connect_timeout=1", zap.NewNop())
	assert.Error(t, err)
}

func TestRunMigrations_InvalidPath(t *testing.T) {
	err := RunMigrations("postgres://localhost:5432/test", "/nonexistent/migrations/path", zap.NewNop())
	assert.Error(t, err)
}
