package clientconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:3200" {
		t.Fatalf("ServerAddress = %q, want %q", cfg.ServerAddress, "localhost:3200")
	}
	if cfg.ConfigDir == "" {
		t.Fatal("ConfigDir is empty")
	}
	if cfg.Insecure {
		t.Fatal("Insecure = true, want false")
	}
}

func TestLoadPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	fileConfigDir := filepath.Join(tempDir, "file-config")
	envConfigDir := filepath.Join(tempDir, "env-config")
	flagConfigDir := filepath.Join(tempDir, "flag-config")
	configPath := filepath.Join(tempDir, "client.json")

	configJSON := `{
  "server_address": "file:3200",
  "config_dir": "` + filepath.ToSlash(fileConfigDir) + `",
  "tls_ca_file": "file-ca.pem",
  "insecure": false
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("GOPHKEEPER_SERVER_ADDRESS", "env:3200")
	t.Setenv("GOPHKEEPER_CONFIG_DIR", envConfigDir)
	t.Setenv("GOPHKEEPER_TLS_CA_FILE", "env-ca.pem")
	t.Setenv("GOPHKEEPER_INSECURE", "true")

	cfg, remainingArgs, err := Load([]string{
		"--config", configPath,
		"--server-address", "flag:3200",
		"--config-dir", flagConfigDir,
		"--tls-ca-file", "flag-ca.pem",
		"--insecure=false",
		"list",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", cfg.ConfigPath, configPath)
	}
	if cfg.ServerAddress != "flag:3200" {
		t.Fatalf("ServerAddress = %q, want %q", cfg.ServerAddress, "flag:3200")
	}
	if cfg.ConfigDir != flagConfigDir {
		t.Fatalf("ConfigDir = %q, want %q", cfg.ConfigDir, flagConfigDir)
	}
	if cfg.TLSCAFile != "flag-ca.pem" {
		t.Fatalf("TLSCAFile = %q, want %q", cfg.TLSCAFile, "flag-ca.pem")
	}
	if cfg.Insecure {
		t.Fatal("Insecure = true, want false")
	}
	if len(remainingArgs) != 1 || remainingArgs[0] != "list" {
		t.Fatalf("remainingArgs = %v, want [list]", remainingArgs)
	}
}
