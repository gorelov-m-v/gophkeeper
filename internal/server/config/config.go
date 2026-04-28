// Package serverconfig provides configuration loading for the GophKeeper server.
package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all server configuration parameters.
type Config struct {
	GRPCAddress       string
	DatabaseDSN       string
	MigrationsPath    string
	JWTSecret         string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	TLSCertFile       string
	TLSKeyFile        string
	AllowInsecureGRPC bool
	ConfigPath        string
}

type fileConfig struct {
	GRPCAddress       *string `json:"grpc_address"`
	DatabaseDSN       *string `json:"database_dsn"`
	MigrationsPath    *string `json:"migrations_path"`
	JWTSecret         *string `json:"jwt_secret"`
	JWTAccessTTL      *string `json:"jwt_access_ttl"`
	JWTRefreshTTL     *string `json:"jwt_refresh_ttl"`
	TLSCertFile       *string `json:"tls_cert_file"`
	TLSKeyFile        *string `json:"tls_key_file"`
	AllowInsecureGRPC *bool   `json:"allow_insecure_grpc"`
}

// Default returns the default server configuration.
func Default() Config {
	return Config{
		GRPCAddress:       "localhost:3200",
		MigrationsPath:    "migrations",
		JWTAccessTTL:      15 * time.Minute,
		JWTRefreshTTL:     168 * time.Hour,
		AllowInsecureGRPC: false,
	}
}

// Load resolves configuration using the precedence order defaults -> JSON -> env -> flags.
func Load(args []string) (*Config, error) {
	cfg := Default()
	cfg.ConfigPath = findConfigPath(args, os.Getenv("CONFIG"))

	if cfg.ConfigPath != "" {
		if err := applyFileConfig(&cfg, cfg.ConfigPath); err != nil {
			return nil, err
		}
	}

	applyEnv(&cfg)

	fs := flag.NewFlagSet("gophkeeper-server", flag.ContinueOnError)
	fs.StringVar(&cfg.GRPCAddress, "grpc-address", cfg.GRPCAddress, "address to listen on")
	fs.StringVar(&cfg.GRPCAddress, "g", cfg.GRPCAddress, "address to listen on")
	fs.StringVar(&cfg.DatabaseDSN, "database-dsn", cfg.DatabaseDSN, "postgres connection string")
	fs.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "postgres connection string")
	fs.StringVar(&cfg.MigrationsPath, "migrations-path", cfg.MigrationsPath, "path to migrations")
	fs.StringVar(&cfg.JWTSecret, "jwt-secret", cfg.JWTSecret, "JWT signing secret")
	fs.DurationVar(&cfg.JWTAccessTTL, "jwt-access-ttl", cfg.JWTAccessTTL, "access token TTL")
	fs.DurationVar(&cfg.JWTRefreshTTL, "jwt-refresh-ttl", cfg.JWTRefreshTTL, "refresh token TTL")
	fs.StringVar(&cfg.TLSCertFile, "tls-cert-file", cfg.TLSCertFile, "path to TLS certificate")
	fs.StringVar(&cfg.TLSKeyFile, "tls-key-file", cfg.TLSKeyFile, "path to TLS private key")
	fs.BoolVar(&cfg.AllowInsecureGRPC, "allow-insecure-grpc", cfg.AllowInsecureGRPC, "allow plaintext gRPC")
	fs.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "path to JSON config file")
	fs.StringVar(&cfg.ConfigPath, "c", cfg.ConfigPath, "path to JSON config file")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func findConfigPath(args []string, envValue string) string {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-c" || args[i] == "--config":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(args[i], "--config="):
			return strings.TrimPrefix(args[i], "--config=")
		}
	}

	return envValue
}

func applyFileConfig(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}

	if fc.GRPCAddress != nil {
		cfg.GRPCAddress = *fc.GRPCAddress
	}
	if fc.DatabaseDSN != nil {
		cfg.DatabaseDSN = *fc.DatabaseDSN
	}
	if fc.MigrationsPath != nil {
		cfg.MigrationsPath = *fc.MigrationsPath
	}
	if fc.JWTSecret != nil {
		cfg.JWTSecret = *fc.JWTSecret
	}
	if fc.JWTAccessTTL != nil {
		ttl, err := time.ParseDuration(*fc.JWTAccessTTL)
		if err != nil {
			return fmt.Errorf("parse jwt_access_ttl: %w", err)
		}
		cfg.JWTAccessTTL = ttl
	}
	if fc.JWTRefreshTTL != nil {
		ttl, err := time.ParseDuration(*fc.JWTRefreshTTL)
		if err != nil {
			return fmt.Errorf("parse jwt_refresh_ttl: %w", err)
		}
		cfg.JWTRefreshTTL = ttl
	}
	if fc.TLSCertFile != nil {
		cfg.TLSCertFile = *fc.TLSCertFile
	}
	if fc.TLSKeyFile != nil {
		cfg.TLSKeyFile = *fc.TLSKeyFile
	}
	if fc.AllowInsecureGRPC != nil {
		cfg.AllowInsecureGRPC = *fc.AllowInsecureGRPC
	}

	return nil
}

func applyEnv(cfg *Config) {
	cfg.GRPCAddress = envOrDefault("GRPC_ADDRESS", cfg.GRPCAddress)
	cfg.DatabaseDSN = firstNonEmpty(os.Getenv("DATABASE_DSN"), os.Getenv("DATABASE_URL"), cfg.DatabaseDSN)
	cfg.MigrationsPath = envOrDefault("MIGRATIONS_PATH", cfg.MigrationsPath)
	cfg.JWTSecret = envOrDefault("JWT_SECRET", cfg.JWTSecret)
	cfg.JWTAccessTTL = durationEnvOrDefault("JWT_ACCESS_TTL", cfg.JWTAccessTTL)
	cfg.JWTRefreshTTL = durationEnvOrDefault("JWT_REFRESH_TTL", cfg.JWTRefreshTTL)
	cfg.TLSCertFile = envOrDefault("TLS_CERT_FILE", cfg.TLSCertFile)
	cfg.TLSKeyFile = envOrDefault("TLS_KEY_FILE", cfg.TLSKeyFile)
	cfg.AllowInsecureGRPC = boolEnvOrDefault("ALLOW_INSECURE_GRPC", cfg.AllowInsecureGRPC)
}

func validate(cfg Config) error {
	if cfg.GRPCAddress == "" {
		return fmt.Errorf("grpc address is required")
	}
	if cfg.DatabaseDSN == "" {
		return fmt.Errorf("database DSN is required")
	}
	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required")
	}
	if len(cfg.JWTSecret) < 16 {
		return fmt.Errorf("JWT secret must be at least 16 bytes")
	}
	if !cfg.AllowInsecureGRPC {
		if cfg.TLSCertFile == "" {
			return fmt.Errorf("TLS certificate file is required when insecure gRPC is disabled")
		}
		if cfg.TLSKeyFile == "" {
			return fmt.Errorf("TLS private key file is required when insecure gRPC is disabled")
		}
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return duration
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconvParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func strconvParseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y":
		return true, nil
	case "0", "f", "false", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}
