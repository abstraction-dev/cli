// Package config reads and writes the CLI's hidden dotfile (~/.abstr.json),
// which holds the current workspace and API key.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultBaseURL is the production API host used when nothing overrides it.
const DefaultBaseURL = "https://app.abstraction.dev"

const (
	fileName      = ".abstr.json"
	envAPIKey     = "ABSTR_API_KEY"
	envWorkspace  = "ABSTR_WORKSPACE"
	envBaseURL    = "ABSTR_API_URL"
	envConfigPath = "ABSTR_CONFIG"
)

// Config is the on-disk CLI configuration.
type Config struct {
	APIKey     string `json:"api_key,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	APIBaseURL string `json:"api_base_url,omitempty"`

	// AutoUpgrade, when true, applies a newer release automatically instead of
	// only printing a notice. See internal/selfupdate.
	AutoUpgrade bool `json:"auto_upgrade,omitempty"`
	// LastUpdateCheck is when the background update check last ran (RFC3339).
	LastUpdateCheck string `json:"last_update_check,omitempty"`
	// LatestSeen is the newest release tag the last check observed (e.g. v1.3.0).
	LatestSeen string `json:"latest_seen,omitempty"`

	path string // where this was loaded from / will be saved to
}

// Path returns the config file location: $ABSTR_CONFIG, else ~/.abstr.json.
func Path() (string, error) {
	if p := os.Getenv(envConfigPath); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fileName), nil
}

// Load reads the config. A missing file yields an empty Config (not an error),
// so first-run bootstrap can proceed. override, when non-empty, wins over the
// default path.
func Load(override string) (*Config, error) {
	path := override
	if path == "" {
		p, err := Path()
		if err != nil {
			return nil, err
		}
		path = p
	}

	cfg := &Config{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.path = path
	return cfg, nil
}

// Save writes the config with 0600 perms (it holds a secret).
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, append(data, '\n'), 0o600)
}

// FilePath returns the resolved config path.
func (c *Config) FilePath() string { return c.path }

// APIKeyResolved returns the effective API key: $ABSTR_API_KEY over the file.
func (c *Config) APIKeyResolved() string {
	if v := os.Getenv(envAPIKey); v != "" {
		return v
	}
	return c.APIKey
}

// WorkspaceResolved returns the effective workspace: $ABSTR_WORKSPACE over the
// file. A flag, when set, takes precedence at the call site.
func (c *Config) WorkspaceResolved() string {
	if v := os.Getenv(envWorkspace); v != "" {
		return v
	}
	return c.Workspace
}

// BaseURLResolved returns the effective API base URL: $ABSTR_API_URL, then the
// file, then the default.
func (c *Config) BaseURLResolved() string {
	if v := os.Getenv(envBaseURL); v != "" {
		return v
	}
	if c.APIBaseURL != "" {
		return c.APIBaseURL
	}
	return DefaultBaseURL
}
