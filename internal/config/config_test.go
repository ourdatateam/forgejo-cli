package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateConfigEnv(t *testing.T, path string) {
	t.Helper()
	t.Setenv("FORGEJO_CONFIG", path)
	for k := range knownKeys {
		t.Setenv(k, "")
	}
}

func writeConfig(t *testing.T, path string, perm os.FileMode, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatalf("chmod config: %v", err)
	}
}

func TestConfigMode0600Enforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	isolateConfigEnv(t, path)
	writeConfig(t, path, 0o644, "FORGEJO_URL=https://forgejo.example\nFORGEJO_TOKEN=file-token\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want insecure permissions error")
	}
	if msg := err.Error(); !strings.Contains(msg, "insecure permissions") || !strings.Contains(msg, "expected 600") {
		t.Fatalf("error = %q, want insecure permissions migration help", msg)
	}
}

func TestConfigSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	target := filepath.Join(dir, "target")
	isolateConfigEnv(t, path)
	writeConfig(t, target, 0o600, "FORGEJO_URL=https://forgejo.example\nFORGEJO_TOKEN=file-token\n")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink config: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %q, want symlink rejection", err.Error())
	}
}

func TestConfigBashDollarQuotingRejectedWithMigrationMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	isolateConfigEnv(t, path)
	writeConfig(t, path, 0o600, "FORGEJO_URL=https://forgejo.example\nFORGEJO_TOKEN=$'file-token'\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want bash quoting error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bash $'") || !strings.Contains(msg, "does not evaluate") || !strings.Contains(msg, "re-enter") {
		t.Fatalf("error = %q, want bash $'...' migration message", msg)
	}
}

func TestConfigQuotedValuesUnwrapped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	isolateConfigEnv(t, path)
	writeConfig(t, path, 0o600, "FORGEJO_URL=\"https://forgejo.example/\"\nFORGEJO_TOKEN='quoted-token'\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.URL != "https://forgejo.example" {
		t.Fatalf("URL = %q, want trimmed quoted URL", cfg.URL)
	}
	if cfg.Token != "quoted-token" {
		t.Fatalf("Token = %q, want unwrapped token", cfg.Token)
	}
}

func TestConfigEnvForgejoTokenBeatsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	isolateConfigEnv(t, path)
	t.Setenv("FORGEJO_TOKEN", "env-token")
	writeConfig(t, path, 0o600, "FORGEJO_URL=https://forgejo.example\nFORGEJO_TOKEN=file-token\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Token != "env-token" {
		t.Fatalf("Token = %q, want env-token", cfg.Token)
	}
}

func TestConfigForgejoTokenCommandRunsAndTrims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	isolateConfigEnv(t, path)
	writeConfig(t, path, 0o600, "FORGEJO_URL=https://forgejo.example\nFORGEJO_TOKEN_COMMAND=printf 'cmd-token\\n\\n'\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Token != "cmd-token" {
		t.Fatalf("Token = %q, want trimmed command output", cfg.Token)
	}
}
