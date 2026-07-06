// Package browser opens URLs in the user's default browser.
package browser

import (
	"os/exec"
	"runtime"
)

// Open launches url in the default browser. It returns an error when no
// launcher is available (e.g. a headless session), so callers can fall back to
// printing the URL.
func Open(url string) error {
	var name string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default: // linux, *bsd
		name, args = "xdg-open", []string{url}
	}

	return exec.Command(name, args...).Start()
}
