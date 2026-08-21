package cli

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/abstraction-dev/cli/internal/config"
	"github.com/abstraction-dev/cli/internal/history"
	"github.com/abstraction-dev/cli/internal/render"
)

// historyListDefault is how many entries `abstr history` shows without -n.
const historyListDefault = 20

// runHistory inspects or clears the locally stored exchanges.
func runHistory(args []string) int {
	fs := flag.NewFlagSet("abstr history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var configPath string
	var limit int
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.IntVar(&limit, "n", historyListDefault, "how many entries to show")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitRuntime
	}

	store, err := openHistory(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitRuntime
	}
	r := newRenderer()

	action := "list"
	if rest := fs.Args(); len(rest) > 0 {
		action = rest[0]
	}

	switch action {
	case "list":
		return listHistory(store, r, limit)
	case "path":
		fmt.Println(store.FilePath())
		return exitOK
	case "clear":
		if err := store.Clear(); err != nil {
			r.Error("abstr: " + err.Error())
			return exitRuntime
		}
		r.Success("History cleared.")
		return exitOK
	default:
		fmt.Fprintln(os.Stderr, "unknown history command: "+action)
		return exitUsage
	}
}

// listHistory prints the newest entries, one row each: when, workspace, and the
// question collapsed to the terminal width.
func listHistory(store *history.Store, r *render.Renderer, limit int) int {
	entries, err := store.Recent(limit)
	if err != nil {
		r.Error("abstr: " + err.Error())
		return exitRuntime
	}

	if len(entries) == 0 {
		r.Info("No history yet.")
		return exitOK
	}

	width := render.TermWidth(os.Stdout) - 32
	for _, e := range entries {
		fmt.Printf("%s  %s  %s\n",
			e.AskedAt.Local().Format(time.RFC3339),
			shortWorkspace(e.Workspace),
			e.Headline(width))
	}
	return exitOK
}

// openHistory returns the history store sized by configuration.
func openHistory(cfg *config.Config) (*history.Store, error) {
	return history.Open(cfg.HistoryLimitResolved())
}

// shortWorkspace trims a workspace slug to its leading segment, which is enough
// to tell rows apart without spending a full UUID per line.
func shortWorkspace(slug string) string {
	if len(slug) <= 8 {
		return slug
	}
	return slug[:8]
}
