package cli

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
)

var verdictsToRule = []string{"fixed", "invalid", "wont_fix", "duplicate", "open"}

// runResolve rules on one issue of a review as the caller, through the
// route the app's review page uses.
func runResolve(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr review resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printUsage(os.Stderr) }

	var opts runOptions
	var ruling apiclient.Ruling
	var reportID string
	fs.StringVar(&opts.workspace, "w", "", "workspace slug")
	fs.StringVar(&opts.workspace, "workspace", "", "workspace slug")
	fs.StringVar(&opts.configPath, "config", "", "config file path")
	fs.StringVar(&opts.apiURL, "api-url", "", "API base URL")
	fs.StringVar(&ruling.Verdict, "as", "", "fixed, invalid, wont_fix, duplicate or open")
	fs.StringVar(&ruling.Reason, "reason", "", "why, required for invalid and wont_fix")
	fs.StringVar(&ruling.DuplicateOf, "duplicate-of", "", "the issue a duplicate repeats")
	fs.StringVar(&reportID, "report", "", "review id (default: the current branch's review that holds the issue)")

	targets, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if len(targets) != 1 {
		fmt.Fprintln(os.Stderr, "abstr: review resolve takes one issue id, e.g. abstr review resolve 4211 --as fixed")
		return exitUsage
	}
	issueID, err := checkRuling(targets[0], &ruling)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitUsage
	}

	env, err := ensureConfigured(ctx, opts, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abstr: "+err.Error())
		return exitCodeFor(err)
	}
	if reportID == "" {
		repository, branch, err := currentBranch()
		if err != nil {
			env.render.Errorf("abstr: %s; pass --report <id>", err.Error())
			return exitUsage
		}
		reportID, err = issueReview(ctx, env.client, env.workspace, repository, branch, issueID)
		if err != nil {
			return reportRunError(ctx, env, err)
		}
	}

	issue, err := env.client.RuleOnIssue(ctx, env.workspace, reportID, issueID, ruling)
	if err != nil {
		return reportRunError(ctx, env, rulingError(err, reportID, issueID))
	}
	printRuling(os.Stdout, issueID, ruling, issue)
	return exitOK
}

var issueIDRe = regexp.MustCompile(`^#?(\d+)$`)

// checkRuling reads an issue id given as 4211 or #4211, and refuses a
// ruling the route would: an unknown verdict, one missing its reason, or an
// original named by anything but a duplicate.
func checkRuling(target string, ruling *apiclient.Ruling) (string, error) {
	id := issueIDRe.FindStringSubmatch(target)
	if id == nil {
		return "", fmt.Errorf("%q is not an issue id; abstr review lists them as #4211", target)
	}
	if !slices.Contains(verdictsToRule, ruling.Verdict) {
		return "", fmt.Errorf("--as takes one of %s", strings.Join(verdictsToRule, ", "))
	}
	if (ruling.Verdict == "invalid" || ruling.Verdict == "wont_fix") && strings.TrimSpace(ruling.Reason) == "" {
		return "", fmt.Errorf("--as %s needs a --reason", ruling.Verdict)
	}
	if (ruling.Verdict == "duplicate") != (ruling.DuplicateOf != "") {
		return "", errors.New("--duplicate-of names the original of a duplicate, and only of one")
	}
	if ruling.DuplicateOf != "" {
		original := issueIDRe.FindStringSubmatch(ruling.DuplicateOf)
		if original == nil {
			return "", fmt.Errorf("--duplicate-of %q is not an issue id", ruling.DuplicateOf)
		}
		ruling.DuplicateOf = original[1]
	}
	return id[1], nil
}

// issueReview is the review of the branch that holds the issue: its pull
// request's in this repository, or one of the caller's pre-reviews of it.
// Pre-reviews name no repository, so the issue is what tells them apart.
func issueReview(ctx context.Context, client *apiclient.Client, workspace, repository, branch, issueID string) (string, error) {
	prs, err := client.PRReviews(ctx, workspace, branch)
	if err != nil {
		return "", err
	}
	pres, err := client.BranchReviews(ctx, workspace)
	if err != nil {
		return "", err
	}
	prPrefix := "github.com/" + strings.ToLower(repository) + "/pull/"
	var candidates []string
	for _, review := range append(prs, pres...) {
		if review.PRHeadRef == branch && (review.IsBranch() || strings.HasPrefix(normalizePRURL(review.PRURL), prPrefix)) {
			candidates = append(candidates, review.ID)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no review of %s in this workspace; run abstr review, or pass --report <id>", branch)
	}
	for _, reportID := range candidates {
		issues, err := client.ReviewIssues(ctx, workspace, reportID)
		if err != nil {
			return "", err
		}
		if slices.ContainsFunc(issues, func(issue apiclient.ReviewIssue) bool { return issue.IssueID == issueID }) {
			return reportID, nil
		}
	}
	return "", fmt.Errorf("no review of %s has issue #%s; pass --report <id>", branch, issueID)
}

// rulingError says what the route's refusals mean for this issue.
func rulingError(err error, reportID, issueID string) error {
	var apiErr *apiclient.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Status {
	case http.StatusForbidden:
		return errors.New("you need to be an editor or admin of the workspace")
	case http.StatusNotFound:
		return fmt.Errorf("review %s has no issue #%s; pass --report <id> if it is in another review", reportID, issueID)
	}
	return err
}

// printRuling prints the issue as the ruling left it: its verdict, who
// ruled and whether the review still reports it.
func printRuling(w io.Writer, issueID string, ruling apiclient.Ruling, issue *apiclient.ReviewIssue) {
	if issue == nil {
		fmt.Fprintf(w, "#%s %s\n", issueID, ruling.Verdict)
		return
	}
	fmt.Fprintf(w, "#%s %s\n", plain(issue.IssueID), plain(issue.Title))
	line := "  " + plain(cmp.Or(issue.Verdict, "open"))
	if issue.VerdictBy != "" {
		line += " by " + plain(issue.VerdictBy)
	}
	fmt.Fprintf(w, "%s · %s\n", line, plain(strings.ToLower(issue.State)))
	if issue.VerdictReason != "" {
		fmt.Fprintf(w, "    %s\n", indent(issue.VerdictReason))
	}
}
