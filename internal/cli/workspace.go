package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/config"
	"github.com/abstraction-dev/cli/internal/render"
)

// runWorkspace manages the current workspace: pick (default), list, or use.
func runWorkspace(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr workspace", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var configPath, apiURL string
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.StringVar(&apiURL, "api-url", "", "API base URL")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	sub := fs.Args()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitRuntime
	}
	baseURL := cfg.BaseURLResolved()
	if apiURL != "" {
		baseURL = apiURL
	}
	r := newRenderer()

	key := cfg.APIKeyResolved()
	if key == "" {
		r.Errorf("not logged in — run `abstr login`")
		return exitAuth
	}
	client := apiclient.New(baseURL, key)

	action := "pick"
	if len(sub) > 0 {
		action = sub[0]
	}

	switch action {
	case "pick":
		ws, err := pickWorkspace(ctx, client, r)
		if err != nil {
			r.Error(err.Error())
			return exitCodeFor(err)
		}
		return saveWorkspace(cfg, r, ws)

	case "list":
		wss, err := client.Workspaces(ctx)
		if err != nil {
			r.Error(err.Error())
			return exitCodeFor(err)
		}
		current := cfg.WorkspaceResolved()
		for _, w := range wss {
			marker := "  "
			if w.Slug == current {
				marker = "* "
			}
			fmt.Printf("%s%s\t%s\n", marker, w.Slug, w.Name)
		}
		return exitOK

	case "use":
		if len(sub) < 2 {
			r.Errorf("usage: abstr workspace use <slug|name>")
			return exitUsage
		}
		// Join the remaining tokens so an unquoted multi-word name (e.g.
		// `workspace use My Team`) still matches by name.
		target := strings.Join(sub[1:], " ")
		wss, err := client.Workspaces(ctx)
		if err != nil {
			r.Error(err.Error())
			return exitCodeFor(err)
		}
		for _, w := range wss {
			if w.Slug == target || strings.EqualFold(w.Name, target) {
				return saveWorkspace(cfg, r, w.Slug)
			}
		}
		r.Errorf("no workspace matching %q", target)
		return exitRuntime

	default:
		r.Errorf("unknown workspace command: %s", action)
		return exitUsage
	}
}

func saveWorkspace(cfg *config.Config, r *render.Renderer, slug string) int {
	prev := cfg.Workspace
	cfg.Workspace = slug
	if err := cfg.Save(); err != nil {
		cfg.Workspace = prev // don't leave the config object ahead of disk
		r.Error("save config: " + err.Error())
		return exitRuntime
	}
	r.Info("workspace set: " + slug)
	return exitOK
}
