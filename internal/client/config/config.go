// Package clientconfig provides configuration loading for the GophKeeper client.
package clientconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds client configuration values.
type Config struct {
	ServerAddress string
	ConfigDir     string
	TLSCAFile     string
	Insecure      bool
	ConfigPath    string
}

type fileConfig struct {
	ServerAddress *string `json:"server_address"`
	ConfigDir     *string `json:"config_dir"`
	TLSCAFile     *string `json:"tls_ca_file"`
	Insecure      *bool   `json:"insecure"`
}

// Default returns the default client configuration.
func Default() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServerAddress: "localhost:3200",
		ConfigDir:     filepath.Join(homeDir, ".gophkeeper"),
		Insecure:      false,
	}, nil
}

// Load resolves configuration using the precedence order defaults -> JSON -> env -> flags.
func Load(args []string) (*Config, []string, error) {
	cfg, err := Default()
	if err != nil {
		return nil, nil, err
	}

	cfg.ConfigPath = findConfigPath(args, os.Getenv("GOPHKEEPER_CONFIG"))

	if cfg.ConfigPath != "" {
		if err := applyFileConfig(&cfg, cfg.ConfigPath); err != nil {
			return nil, nil, err
		}
	}

	applyEnv(&cfg)

	fs := flag.NewFlagSet("gophkeeper-client", flag.ContinueOnError)
	fs.StringVar(&cfg.ServerAddress, "server-address", cfg.ServerAddress, "gRPC server address")
	fs.StringVar(&cfg.ConfigDir, "config-dir", cfg.ConfigDir, "client configuration directory")
	fs.StringVar(&cfg.TLSCAFile, "tls-ca-file", cfg.TLSCAFile, "path to CA bundle for server TLS")
	fs.BoolVar(&cfg.Insecure, "insecure", cfg.Insecure, "allow plaintext gRPC")
	fs.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "path to JSON config file")
	fs.StringVar(&cfg.ConfigPath, "c", cfg.ConfigPath, "path to JSON config file")
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(cfg.ConfigDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("create config dir: %w", err)
	}

	return &cfg, fs.Args(), nil
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

	if fc.ServerAddress != nil {
		cfg.ServerAddress = *fc.ServerAddress
	}
	if fc.ConfigDir != nil {
		cfg.ConfigDir = *fc.ConfigDir
	}
	if fc.TLSCAFile != nil {
		cfg.TLSCAFile = *fc.TLSCAFile
	}
	if fc.Insecure != nil {
		cfg.Insecure = *fc.Insecure
	}

	return nil
}

func applyEnv(cfg *Config) {
	cfg.ServerAddress = firstNonEmpty(os.Getenv("GOPHKEEPER_SERVER_ADDRESS"), os.Getenv("GOPHKEEPER_SERVER"), cfg.ServerAddress)
	cfg.ConfigDir = firstNonEmpty(os.Getenv("GOPHKEEPER_CONFIG_DIR"), cfg.ConfigDir)
	cfg.TLSCAFile = firstNonEmpty(os.Getenv("GOPHKEEPER_TLS_CA_FILE"), cfg.TLSCAFile)
	cfg.Insecure = boolEnvOrDefault("GOPHKEEPER_INSECURE", cfg.Insecure)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolEnvOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y":
		return true
	case "0", "f", "false", "no", "n":
		return false
	default:
		return fallback
	}
}
