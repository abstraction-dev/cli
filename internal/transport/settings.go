package transport

import (
	"strings"

	"github.com/abstraction-dev/cli/internal/browser"
	"github.com/abstraction-dev/cli/internal/config"
)

// SettingsURL is the page where a user creates an API key.
func SettingsURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/settings"
}

// OpenSettings launches the settings page in the user's browser, honouring a
// configured browser command. Callers fall back to printing the URL on error,
// since a headless session has no launcher.
func OpenSettings(cfg *config.Config, baseURL string) error {
	return browser.OpenWithConfig(cfg, SettingsURL(baseURL))
}
