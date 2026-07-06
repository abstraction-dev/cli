package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.APIKey != "" || cfg.Workspace != "" {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".abstr.json")
	orig := &Config{APIKey: "abs_secret", Workspace: "ws-uuid", path: path}
	if err := orig.Save(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected 0600 perms (holds a secret), got %o", perm)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "abs_secret" || got.Workspace != "ws-uuid" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestResolvedPrecedence(t *testing.T) {
	cfg := &Config{APIKey: "file_key", Workspace: "file_ws"}

	t.Setenv(envAPIKey, "env_key")
	t.Setenv(envWorkspace, "env_ws")
	t.Setenv(envBaseURL, "https://env.example.com")

	if got := cfg.APIKeyResolved(); got != "env_key" {
		t.Fatalf("env should override file for api key, got %q", got)
	}
	if got := cfg.WorkspaceResolved(); got != "env_ws" {
		t.Fatalf("env should override file for workspace, got %q", got)
	}
	if got := cfg.BaseURLResolved(); got != "https://env.example.com" {
		t.Fatalf("env should override for base url, got %q", got)
	}
}

func TestBaseURLDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.BaseURLResolved(); got != DefaultBaseURL {
		t.Fatalf("expected default base url, got %q", got)
	}
}
