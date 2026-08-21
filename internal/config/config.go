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

// DefaultHistoryLimit is how many exchanges local history keeps by default.
const DefaultHistoryLimit = 50

const (
	fileName      = ".abstr.json"
	envAPIKey     = "ABSTR_API_KEY"
	envWorkspace  = "ABSTR_WORKSPACE"
	envBaseURL    = "ABSTR_API_URL"
	envConfigPath = "ABSTR_CONFIG"
	envBrowser    = "ABSTR_BROWSER"
)

// Config is the on-disk CLI configuration.
type Config struct {
	APIKey     string `json:"api_key,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	APIBaseURL string `json:"api_base_url,omitempty"`

	// BrowserCommand overrides how URLs are opened (e.g. "firefox"). Empty
	// means the platform default launcher.
	BrowserCommand string `json:"browser_command,omitempty"`
	// HistoryLimit caps how many exchanges the local history file keeps. Zero
	// means DefaultHistoryLimit.
	HistoryLimit int `json:"history_limit,omitempty"`

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

// BrowserCommandResolved returns the effective browser command: $ABSTR_BROWSER
// over the file. Empty means use the platform default launcher.
func (c *Config) BrowserCommandResolved() string {
	if v := os.Getenv(envBrowser); v != "" {
		return v
	}
	return c.BrowserCommand
}

// HistoryLimitResolved returns the effective history cap, falling back to
// DefaultHistoryLimit. A negative limit disables history entirely.
func (c *Config) HistoryLimitResolved() int {
	if c.HistoryLimit == 0 {
		return DefaultHistoryLimit
	}
	return c.HistoryLimit
}
