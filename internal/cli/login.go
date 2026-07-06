package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/config"
)

// runLogin stores an API key (browser + paste) and picks a workspace.
func runLogin(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var configPath, apiURL string
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.StringVar(&apiURL, "api-url", "", "API base URL")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitRuntime
	}

	baseURL := cfg.BaseURLResolved()
	if apiURL != "" {
		baseURL = apiURL
		cfg.APIBaseURL = apiURL
	}
	r := newRenderer()

	key, err := bootstrapAPIKey(ctx, r, baseURL)
	if err != nil {
		r.Error(err.Error())
		return exitAuth
	}
	cfg.APIKey = key

	ws, err := pickWorkspace(ctx, apiclient.New(baseURL, key), r)
	if err != nil {
		r.Error(err.Error())
		return exitRuntime
	}
	cfg.Workspace = ws

	if err := cfg.Save(); err != nil {
		r.Error("save config: " + err.Error())
		return exitRuntime
	}
	r.Info("Logged in. Config saved to " + cfg.FilePath())
	return exitOK
}
