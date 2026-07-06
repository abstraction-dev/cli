package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/abstraction-dev/cli/internal/config"
)

// runConfig inspects the CLI configuration.
func runConfig(args []string) int {
	fs := flag.NewFlagSet("abstr config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var configPath string
	fs.StringVar(&configPath, "config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	sub := fs.Args()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitRuntime
	}

	action := "show"
	if len(sub) > 0 {
		action = sub[0]
	}

	switch action {
	case "path":
		fmt.Println(cfg.FilePath())
	case "show":
		fmt.Printf("config:     %s\n", cfg.FilePath())
		fmt.Printf("workspace:  %s\n", cfg.Workspace)
		fmt.Printf("api_key:    %s\n", redactKey(cfg.APIKey))
		fmt.Printf("api_url:    %s\n", cfg.BaseURLResolved())
	default:
		fmt.Fprintln(os.Stderr, "unknown config command: "+action)
		return exitUsage
	}
	return exitOK
}

func redactKey(key string) string {
	switch {
	case key == "":
		return "(not set)"
	case len(key) <= 8:
		return "****"
	default:
		return key[:8] + "…"
	}
}
