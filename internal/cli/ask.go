package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/render"
)

type queryMode int

const (
	modeQuery queryMode = iota
	modeInteractive
	modeEmpty
)

// runAsk parses flags, determines the mode (immediate vs interactive), and
// dispatches.
func runAsk(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printUsage(os.Stderr) }

	var opts runOptions
	fs.StringVar(&opts.workspace, "w", "", "workspace slug")
	fs.StringVar(&opts.workspace, "workspace", "", "workspace slug")
	fs.StringVar(&opts.pr, "pr", "", "GitHub pull request URL")
	fs.StringVar(&opts.configPath, "config", "", "config file path")
	fs.StringVar(&opts.apiURL, "api-url", "", "API base URL")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}

	if opts.pr != "" && !isPRURL(opts.pr) {
		fmt.Fprintln(os.Stderr, "abstr: -pr must be a GitHub pull request URL (e.g. https://github.com/owner/repo/pull/123)")
		return exitUsage
	}

	query, mode := resolveQuery(fs.Args())
	if mode == modeEmpty {
		fmt.Fprintln(os.Stderr, `abstr: no question provided (usage: abstr [flags] "your question")`)
		return exitUsage
	}

	env, err := ensureConfigured(ctx, opts, mode == modeInteractive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitCodeFor(err)
	}

	// Check for a newer release concurrently with the task; the notice or
	// auto-upgrade is emitted once the task completes.
	uc := startUpdateCheck(ctx, env.cfg)
	defer finish(ctx, uc)

	if mode == modeInteractive {
		return runREPL(env, opts.pr)
	}

	turnCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	return runImmediate(turnCtx, env, query, opts.pr)
}

// resolveQuery picks the query source: a positional argument, else piped stdin,
// else (on a TTY) interactive mode.
func resolveQuery(posArgs []string) (string, queryMode) {
	if len(posArgs) > 0 {
		return strings.Join(posArgs, " "), modeQuery
	}

	if piped, data := readPipedStdin(); piped {
		q := strings.TrimSpace(data)
		if q == "" {
			return "", modeEmpty
		}
		return q, modeQuery
	}

	if render.IsTerminal(os.Stdin) {
		return "", modeInteractive
	}
	return "", modeEmpty
}

func readPipedStdin() (bool, string) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false, ""
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return false, "" // interactive terminal, not a pipe
	}
	data, _ := io.ReadAll(os.Stdin)
	return true, string(data)
}

// runImmediate posts one ask and prints the whole answer, then exits. Immediate
// mode is a one-shot: buffered output is what pipes/scripts want, and on a TTY it
// renders the full answer as markdown. Streaming lives in the interactive REPL.
func runImmediate(ctx context.Context, env *appEnv, query, pr string) int {
	req := apiclient.AskRequest{Workspace: env.workspace, Question: query, PR: pr}

	// A one-shot never continues, so the conversation the reply names is not kept.
	res, err := env.client.AskBuffered(ctx, req)
	if err != nil {
		return reportRunError(ctx, env, err)
	}
	ans := res.Answer

	// Render markdown to ANSI on a terminal; keep raw markdown when piped so the
	// output stays clean for downstream tools.
	if render.IsTerminal(os.Stdout) {
		env.render.Output(render.Markdown(ans, render.TermWidth(os.Stdout)))
		return exitOK
	}
	env.render.Output(ans)
	if !strings.HasSuffix(ans, "\n") {
		env.render.Output("\n")
	}
	return exitOK
}

func reportRunError(ctx context.Context, env *appEnv, err error) int {
	if ctx.Err() != nil {
		env.render.Errorf("cancelled")
		return exitInterrupt
	}
	env.render.Errorf("abstr: %s", err.Error())
	return exitCodeFor(err)
}
