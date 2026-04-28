package serverconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRequiresDatabaseDSN(t *testing.T) {
	_, err := Load([]string{"--allow-insecure-grpc=true"})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if err.Error() != "database DSN is required" {
		t.Fatalf("Load() error = %q, want %q", err.Error(), "database DSN is required")
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	_, err := Load([]string{
		"--database-dsn", "postgres://localhost/test",
		"--jwt-secret", "short",
		"--allow-insecure-grpc=true",
	})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if err.Error() != "JWT secret must be at least 16 bytes" {
		t.Fatalf("Load() error = %q, want %q", err.Error(), "JWT secret must be at least 16 bytes")
	}
}

func TestLoadRequiresTLSFilesWhenSecure(t *testing.T) {
	_, err := Load([]string{
		"--database-dsn", "postgres://localhost/test",
		"--jwt-secret", "supersecretkey123",
	})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if err.Error() != "TLS certificate file is required when insecure gRPC is disabled" {
		t.Fatalf("Load() error = %q, want %q", err.Error(), "TLS certificate file is required when insecure gRPC is disabled")
	}
}

func TestLoadPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "server.json")
	configJSON := `{
  "grpc_address": "file:3200",
  "database_dsn": "postgres://file/test",
  "migrations_path": "file-migrations",
  "jwt_secret": "file-secret-123456",
  "jwt_access_ttl": "30m",
  "jwt_refresh_ttl": "96h",
  "tls_cert_file": "file-cert.pem",
  "tls_key_file": "file-key.pem",
  "allow_insecure_grpc": false
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("GRPC_ADDRESS", "env:3200")
	t.Setenv("DATABASE_DSN", "postgres://env/test")
	t.Setenv("MIGRATIONS_PATH", "env-migrations")
	t.Setenv("JWT_SECRET", "env-secret-123456")
	t.Setenv("JWT_ACCESS_TTL", "45m")
	t.Setenv("JWT_REFRESH_TTL", "120h")
	t.Setenv("ALLOW_INSECURE_GRPC", "true")

	cfg, err := Load([]string{
		"--config", configPath,
		"--grpc-address", "flag:3200",
		"--database-dsn", "postgres://flag/test",
		"--migrations-path", "flag-migrations",
		"--jwt-secret", "flag-secret-123456",
		"--jwt-access-ttl", "15m",
		"--jwt-refresh-ttl", "72h",
		"--allow-insecure-grpc=true",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, configPath)
	}
	if cfg.GRPCAddress != "flag:3200" {
		t.Fatalf("GRPCAddress = %q, want %q", cfg.GRPCAddress, "flag:3200")
	}
	if cfg.DatabaseDSN != "postgres://flag/test" {
		t.Fatalf("DatabaseDSN = %q, want %q", cfg.DatabaseDSN, "postgres://flag/test")
	}
	if cfg.MigrationsPath != "flag-migrations" {
		t.Fatalf("MigrationsPath = %q, want %q", cfg.MigrationsPath, "flag-migrations")
	}
	if cfg.JWTSecret != "flag-secret-123456" {
		t.Fatalf("JWTSecret = %q, want %q", cfg.JWTSecret, "flag-secret-123456")
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Fatalf("JWTAccessTTL = %v, want %v", cfg.JWTAccessTTL, 15*time.Minute)
	}
	if cfg.JWTRefreshTTL != 72*time.Hour {
		t.Fatalf("JWTRefreshTTL = %v, want %v", cfg.JWTRefreshTTL, 72*time.Hour)
	}
	if !cfg.AllowInsecureGRPC {
		t.Fatal("AllowInsecureGRPC = false, want true")
	}
}
