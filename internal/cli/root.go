// Package cli implements the abstr command surface: an interface to Astrid /
// Agent chat over the backend's /api/cli/ask endpoint.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

// Main is the CLI entry point. It returns the process exit code.
func Main(args []string) int {
	ctx := context.Background()

	if len(args) > 0 {
		switch args[0] {
		case "login":
			return runLogin(ctx, args[1:])
		case "workspace", "ws":
			return runWorkspace(ctx, args[1:])
		case "config":
			return runConfig(args[1:])
		case "help", "-h", "--help":
			printUsage(os.Stdout)
			return exitOK
		case "version", "-v", "--version":
			fmt.Println("abstr " + version)
			return exitOK
		}
	}

	return runAsk(ctx, args)
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `abstr — chat with Astrid from your terminal

Usage:
  abstr [flags] [question]            Ask a question. With no question on a TTY, opens an interactive session.
  echo "question" | abstr             Ask via piped stdin.
  abstr login                         Store your API key and pick a workspace.
  abstr workspace [list|use <slug>]   Manage the current workspace.
  abstr config [path|show]            Inspect CLI configuration.

Flags:
  -w, -workspace <slug>   Workspace to query (a UUID slug).
  -pr <url>               Scope the question to a GitHub pull request URL.
  -api-url <url>          Backend base URL (default https://app.abstraction.dev).
  -config <path>          Config file (default ~/.abstr.json).
  -h                      Show this help.

Interactive commands: /pr [url], /pr clear, /workspace [slug], /new, /help, /exit
`)
}
