package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/render"

	"github.com/charmbracelet/x/ansi"
)

// runReview reviews the working directory's current branch, or the pull
// request named by URL, and follows the review until it finishes.
func runReview(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr review", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printUsage(os.Stderr) }

	var opts runOptions
	var base, follow string
	var noWait, asJSON bool
	fs.StringVar(&opts.workspace, "w", "", "workspace slug")
	fs.StringVar(&opts.workspace, "workspace", "", "workspace slug")
	fs.StringVar(&opts.configPath, "config", "", "config file path")
	fs.StringVar(&opts.apiURL, "api-url", "", "API base URL")
	fs.StringVar(&base, "base", "", "branch a pre-review compares against")
	fs.StringVar(&follow, "follow", "", "review id to follow")
	fs.BoolVar(&noWait, "no-wait", false, "start the review and return")
	fs.BoolVar(&asJSON, "json", false, "emit NDJSON")

	targets, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if len(targets) > 1 || (follow != "" && len(targets) > 0) {
		fmt.Fprintln(os.Stderr, "abstr: review takes one pull request URL, or none to review the current branch")
		return exitUsage
	}

	env, err := ensureConfigured(ctx, opts, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitCodeFor(err)
	}

	reportID := follow
	if reportID == "" {
		start, err := reviewTarget(targets, base, env.render)
		if err != nil {
			env.render.Errorf("abstr: %s", err.Error())
			return exitUsage
		}
		started, err := env.client.StartReview(ctx, env.workspace, start)
		if err != nil {
			return reportRunError(ctx, env, err)
		}
		if asJSON {
			data, _ := json.Marshal(started)
			writeNDJSON(os.Stdout, "started", data)
		}
		if started.ReportID == "" {
			env.render.Info("The review starts once the new head is indexed; run abstr review again in a few minutes.")
			return exitOK
		}
		reportID = started.ReportID
		if noWait {
			env.render.Info("Started review " + reportID + ". Follow it with: abstr review --follow " + reportID)
			return exitOK
		}
	}

	followCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	return followReview(followCtx, env, reportID, asJSON)
}

// parseInterspersed parses flags placed before, between or after the
// positional arguments, which it returns.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// reviewTarget is the PR URL given, or else the checkout's current branch,
// warning when it has commits GitHub hasn't seen.
func reviewTarget(targets []string, base string, r *render.Renderer) (apiclient.StartReviewRequest, error) {
	if len(targets) == 1 {
		if !isPRURL(targets[0]) {
			return apiclient.StartReviewRequest{}, fmt.Errorf("%q is not a pull request URL (e.g. https://github.com/owner/repo/pull/123)", targets[0])
		}
		return apiclient.StartReviewRequest{PRURL: targets[0]}, nil
	}

	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return apiclient.StartReviewRequest{}, fmt.Errorf("not in a git checkout: %w", err)
	}
	if branch == "HEAD" {
		return apiclient.StartReviewRequest{}, errors.New("HEAD is detached; check out the branch to review")
	}
	origin, err := git("remote", "get-url", "origin")
	if err != nil {
		return apiclient.StartReviewRequest{}, errors.New("the checkout has no origin remote")
	}
	repository, ok := githubRepository(origin)
	if !ok {
		return apiclient.StartReviewRequest{}, fmt.Errorf("origin %s is not a GitHub repository", origin)
	}

	if ahead, err := git("rev-list", "--count", "origin/"+branch+"..HEAD"); err != nil {
		r.Warn("branch " + branch + " isn't on origin; the review reads GitHub, so push it first")
	} else if n, _ := strconv.Atoi(ahead); n > 0 {
		r.Warn(fmt.Sprintf("%d local commit(s) aren't pushed; the review reads GitHub and won't see them", n))
	}
	return apiclient.StartReviewRequest{Repository: repository, Branch: branch, BaseBranch: base}, nil
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

var githubRemoteRe = regexp.MustCompile(`^(?:[a-z+]+://)?(?:[^@/]+@)?github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$`)

// githubRepository reads owner/name from a GitHub remote URL, in its HTTPS
// or SSH form.
func githubRepository(remote string) (string, bool) {
	m := githubRemoteRe.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}

// followReview shows the review's progress until it finishes, then its
// verdict and open findings. An interrupt detaches and leaves it running.
func followReview(ctx context.Context, env *appEnv, reportID string, asJSON bool) int {
	view := newProgressView(os.Stderr, render.IsTerminal(os.Stderr), render.TermWidth(os.Stderr))
	onProgress := view.update
	if asJSON {
		onProgress = func(progress apiclient.ReviewProgress) { writeNDJSON(os.Stdout, "progress", progress.Raw) }
	}

	finished, err := env.client.FollowReview(ctx, env.workspace, reportID, onProgress)
	if ctx.Err() != nil {
		env.render.Info("Detached; the review keeps running. Reattach with: abstr review --follow " + reportID)
		return exitInterrupt
	}
	if err != nil {
		return reportRunError(ctx, env, err)
	}

	issues, err := env.client.ReviewIssues(ctx, env.workspace, reportID)
	if err != nil {
		env.render.Warn("could not load the findings: " + err.Error())
	}
	var open []apiclient.ReviewIssue
	for _, issue := range issues {
		if issue.Open() {
			open = append(open, issue)
		}
	}

	if asJSON {
		writeNDJSON(os.Stdout, "finished", finished.Raw)
		findings, _ := json.Marshal(open)
		writeNDJSON(os.Stdout, "findings", findings)
	} else {
		printFindings(os.Stdout, finished, open)
	}
	if finished.Status != "COMPLETED" {
		return exitRuntime
	}
	return exitOK
}

func writeNDJSON(w io.Writer, event string, data json.RawMessage) {
	line, _ := json.Marshal(struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}{event, data})
	fmt.Fprintf(w, "%s\n", line)
}

// printFindings prints how the review ended and the findings it raises.
func printFindings(w io.Writer, finished apiclient.ReviewFinished, open []apiclient.ReviewIssue) {
	summary := "Review " + prStatusLabel(finished.Status)
	if finished.Verdict != "" {
		summary += " · " + finished.Verdict
	}
	fmt.Fprintf(w, "%s · %d open finding(s)\n", summary, len(open))
	if finished.URL != "" {
		fmt.Fprintln(w, finished.URL)
	}
	for _, issue := range open {
		location := issue.FilePath
		if issue.StartLine > 0 {
			location += ":" + strconv.Itoa(issue.StartLine)
		}
		title := issue.Title
		if title == "" {
			title = issue.Description
		}
		fmt.Fprintf(w, "\n  [%s] %s\n    %s\n", cmp.Or(issue.Severity, issue.Category), location, title)
		if issue.Title != "" && issue.Description != "" {
			fmt.Fprintf(w, "    %s\n", issue.Description)
		}
	}
}

// progressView draws a review's progress: on a terminal its step list,
// redrawn in place and cut to the terminal's width so no line wraps;
// elsewhere one line per change.
type progressView struct {
	w     io.Writer
	tty   bool
	width int
	drawn int
	last  string
}

func newProgressView(w io.Writer, tty bool, width int) *progressView {
	return &progressView{w: w, tty: tty, width: width}
}

func (v *progressView) update(progress apiclient.ReviewProgress) {
	if !v.tty {
		line := fmt.Sprintf("%3d%% %s", int(progress.Fraction*100), progress.Headline)
		if line != v.last {
			fmt.Fprintln(v.w, line)
			v.last = line
		}
		return
	}

	lines := []string{fmt.Sprintf("%s  %d%%", progress.Headline, int(progress.Fraction*100))}
	for _, step := range progress.Steps {
		lines = append(lines, "  "+stepLine(step))
		for _, substep := range step.Substeps {
			lines = append(lines, "      "+stepLine(substep))
		}
	}
	if v.width > 0 {
		for i, line := range lines {
			lines[i] = ansi.Truncate(line, v.width-1, "…")
		}
	}
	if v.drawn > 0 {
		fmt.Fprintf(v.w, "\x1b[%dF\x1b[J", v.drawn)
	}
	fmt.Fprintln(v.w, strings.Join(lines, "\n"))
	v.drawn = len(lines)
}

var stepIcons = map[string]string{
	"completed": "✓",
	"running":   "●",
	"failed":    "✗",
	"skipped":   "–",
}

// stepLine is a step's state, name, and its count of work, or else its
// detail. A count of one job says nothing and is left out.
func stepLine(step apiclient.ReviewStep) string {
	line := cmp.Or(stepIcons[step.State], "·") + " " + step.Name
	if step.Done != nil && step.Total != nil && *step.Total > 1 {
		return line + fmt.Sprintf(" (%d/%d)", *step.Done, *step.Total)
	}
	if step.Detail != "" {
		line += " · " + step.Detail
	}
	return line
}
