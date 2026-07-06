package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/browser"
	"github.com/abstraction-dev/cli/internal/config"
	"github.com/abstraction-dev/cli/internal/render"

	"golang.org/x/term"
)

// Exit codes.
const (
	exitOK        = 0
	exitRuntime   = 1
	exitUsage     = 2
	exitAuth      = 3
	exitInterrupt = 130
)

// runOptions are the flags shared by the ask flow.
type runOptions struct {
	configPath string
	workspace  string
	apiURL     string
	pr         string
}

// appEnv is a ready-to-use, authenticated CLI environment.
type appEnv struct {
	cfg       *config.Config
	client    *apiclient.Client
	render    *render.Renderer
	workspace string
}

func newRenderer() *render.Renderer {
	return render.New(os.Stdout, os.Stderr, nil)
}

// ensureConfigured loads config, ensures an API key (bootstrapping on first
// use) and a workspace, and returns a ready appEnv. When interactive, a missing
// workspace is resolved via a picker; otherwise a missing workspace is an error.
func ensureConfigured(ctx context.Context, opts runOptions, interactive bool) (*appEnv, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	baseURL := cfg.BaseURLResolved()
	if opts.apiURL != "" {
		baseURL = opts.apiURL
	}

	r := newRenderer()

	apiKey := cfg.APIKeyResolved()
	if apiKey == "" {
		key, err := bootstrapAPIKey(ctx, r, baseURL)
		if err != nil {
			return nil, err
		}
		cfg.APIKey = key
		if err := cfg.Save(); err != nil {
			r.Warn("could not save config: " + err.Error())
		}
		apiKey = key
	}

	client := apiclient.New(baseURL, apiKey)

	workspace := cfg.WorkspaceResolved()
	if opts.workspace != "" {
		workspace = opts.workspace
	}
	if workspace == "" {
		if !interactive {
			return nil, errors.New("no workspace configured; pass -w <slug> or run `abstr login`")
		}
		ws, err := pickWorkspace(ctx, client, r)
		if err != nil {
			return nil, err
		}
		workspace = ws
		cfg.Workspace = ws
		if err := cfg.Save(); err != nil {
			r.Warn("could not save config: " + err.Error())
		}
	}

	return &appEnv{cfg: cfg, client: client, render: r, workspace: workspace}, nil
}

// bootstrapAPIKey walks the first-run flow: open the settings page, prompt for
// a pasted key, and validate it against the API before returning.
func bootstrapAPIKey(ctx context.Context, r *render.Renderer, baseURL string) (string, error) {
	settingsURL := strings.TrimRight(baseURL, "/") + "/settings"
	r.Info("No API key configured.")
	r.Info("Opening " + settingsURL + " — create an API key there, then paste it below.")
	if err := browser.Open(settingsURL); err != nil {
		r.Info("(couldn't open a browser automatically — open the URL above manually)")
	}

	for attempt := 0; attempt < 3; attempt++ {
		key, err := promptSecret("Paste your API key: ")
		if err != nil {
			return "", err
		}
		if key == "" {
			r.Warn("no key entered")
			continue
		}

		if _, err := apiclient.New(baseURL, key).Workspaces(ctx); err != nil {
			var apiErr *apiclient.APIError
			if errors.As(err, &apiErr) && apiErr.IsAuth() {
				r.Warn("that key was rejected — try again")
				continue
			}
			return "", err
		}
		return key, nil
	}
	return "", errors.New("could not validate an API key")
}

// pickWorkspace lists the user's workspaces and returns the chosen slug. With a
// single workspace it auto-selects; with several it prompts.
func pickWorkspace(ctx context.Context, client *apiclient.Client, r *render.Renderer) (string, error) {
	wss, err := client.Workspaces(ctx)
	if err != nil {
		return "", err
	}
	if len(wss) == 0 {
		return "", errors.New("no workspaces found for this account")
	}
	if len(wss) == 1 {
		r.Info("Using workspace: " + wss[0].Name)
		return wss[0].Slug, nil
	}

	r.Info("Select a workspace:")
	for i, w := range wss {
		suffix := ""
		if w.IsDefault {
			suffix = " (default)"
		}
		r.Info(fmt.Sprintf("  %d) %s%s", i+1, w.Name, suffix))
	}
	for {
		choice, err := promptLine(fmt.Sprintf("Workspace [1-%d]: ", len(wss)))
		if err != nil {
			return "", err
		}
		if n, err := strconv.Atoi(strings.TrimSpace(choice)); err == nil && n >= 1 && n <= len(wss) {
			return wss[n-1].Slug, nil
		}
		r.Warn("invalid choice")
	}
}

// promptSecret reads a line without echo from the controlling terminal, so a
// pasted key never renders. Falls back to stdin when /dev/tty is unavailable.
func promptSecret(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return readLine(os.Stdin, prompt)
	}
	defer tty.Close()

	fmt.Fprint(tty, prompt)
	b, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// promptLine reads a visible line from the controlling terminal (fallback stdin).
func promptLine(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return readLine(os.Stdin, prompt)
	}
	defer tty.Close()

	fmt.Fprint(tty, prompt)
	return readLine(tty, "")
}

func readLine(f *os.File, prompt string) (string, error) {
	if prompt != "" {
		fmt.Fprint(os.Stderr, prompt)
	}
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// exitCodeFor maps an error to a process exit code.
func exitCodeFor(err error) int {
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) && apiErr.IsAuth() {
		return exitAuth
	}
	return exitRuntime
}
