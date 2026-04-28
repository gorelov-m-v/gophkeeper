package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	serverconfig "github.com/user/gophkeeper/internal/server/config"
)

func TestNew_InvalidDSN(t *testing.T) {
	cfg := &serverconfig.Config{
		GRPCAddress:       "localhost:0",
		DatabaseDSN:       "postgres://invalid:invalid@localhost:59999/nonexistent?sslmode=disable&connect_timeout=1",
		JWTSecret:         "test-secret-key-123",
		JWTAccessTTL:      15 * time.Minute,
		JWTRefreshTTL:     168 * time.Hour,
		MigrationsPath:    "migrations",
		AllowInsecureGRPC: true,
	}

	app, err := New(cfg, zap.NewNop())
	assert.Error(t, err, "New should fail with an invalid database URL")
	assert.Nil(t, app)
}
