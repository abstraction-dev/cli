package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/abstraction-dev/cli/internal/apiclient"
)

var verdictsToRule = []string{"fixed", "invalid", "wont_fix", "duplicate", "open"}

// runResolve rules on one finding of a review as the caller, through the
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
	fs.StringVar(&ruling.DuplicateOf, "duplicate-of", "", "the finding a duplicate repeats")
	fs.StringVar(&reportID, "report", "", "review id (default: the newest review of the current branch)")

	targets, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if len(targets) != 1 {
		fmt.Fprintln(os.Stderr, "abstr: review resolve takes one finding id, e.g. abstr review resolve 4211 --as fixed")
		return exitUsage
	}
	findingID, err := checkRuling(targets[0], &ruling)
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
		reportID, err = branchReview(ctx, env.client, env.workspace, repository, branch)
		if err != nil {
			return reportRunError(ctx, env, err)
		}
	}

	issue, err := env.client.RuleOnFinding(ctx, env.workspace, reportID, findingID, ruling)
	if err != nil {
		return reportRunError(ctx, env, rulingError(err, reportID, findingID))
	}
	printRuling(os.Stdout, findingID, ruling, issue)
	return exitOK
}

var findingIDRe = regexp.MustCompile(`^#?(\d+)$`)

// checkRuling reads a finding id given as 4211 or #4211, and refuses a
// ruling the route would: an unknown verdict, one missing its reason, or an
// original named by anything but a duplicate.
func checkRuling(target string, ruling *apiclient.Ruling) (string, error) {
	id := findingIDRe.FindStringSubmatch(target)
	if id == nil {
		return "", fmt.Errorf("%q is not a finding id; abstr review lists them as #4211", target)
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
		original := findingIDRe.FindStringSubmatch(ruling.DuplicateOf)
		if original == nil {
			return "", fmt.Errorf("--duplicate-of %q is not a finding id", ruling.DuplicateOf)
		}
		ruling.DuplicateOf = original[1]
	}
	return id[1], nil
}

// branchReview is the newest review of the branch in the workspace: its
// pull request's, or the caller's pre-review of it. Pre-reviews name no
// repository, so ones of the branch in several repositories are refused.
func branchReview(ctx context.Context, client *apiclient.Client, workspace, repository, branch string) (string, error) {
	prs, err := client.PRReviews(ctx, workspace, branch)
	if err != nil {
		return "", err
	}
	pres, err := client.BranchReviews(ctx, workspace)
	if err != nil {
		return "", err
	}
	prPrefix := "github.com/" + strings.ToLower(repository) + "/pull/"
	var newest int64
	preReviewed := false
	for _, review := range append(prs, pres...) {
		if review.PRHeadRef != branch || (!review.IsBranch() && !strings.HasPrefix(normalizePRURL(review.PRURL), prPrefix)) {
			continue
		}
		if review.IsBranch() {
			if preReviewed {
				return "", fmt.Errorf("you have pre-reviews of %s in several repositories; pass --report <id>", branch)
			}
			preReviewed = true
		}
		if id, _ := strconv.ParseInt(review.ID, 10, 64); id > newest {
			newest = id
		}
	}
	if newest == 0 {
		return "", fmt.Errorf("no review of %s in this workspace; run abstr review, or pass --report <id>", branch)
	}
	return strconv.FormatInt(newest, 10), nil
}

// rulingError says what the route's refusals mean for this finding.
func rulingError(err error, reportID, findingID string) error {
	var apiErr *apiclient.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Status {
	case http.StatusForbidden:
		return errors.New("you need to be an editor or admin of the workspace")
	case http.StatusNotFound:
		return fmt.Errorf("review %s has no finding #%s", reportID, findingID)
	}
	return err
}

// printRuling prints the finding as the ruling left it: its verdict, who
// ruled and whether the review still reports it.
func printRuling(w io.Writer, findingID string, ruling apiclient.Ruling, issue *apiclient.ReviewIssue) {
	if issue == nil {
		fmt.Fprintf(w, "#%s %s\n", findingID, ruling.Verdict)
		return
	}
	fmt.Fprintf(w, "#%s %s\n", plain(issue.CommentID), plain(issue.Title))
	line := "  " + plain(issue.Verdict)
	if issue.VerdictBy != "" {
		line += " by " + plain(issue.VerdictBy)
	}
	fmt.Fprintf(w, "%s · %s\n", line, plain(strings.ToLower(issue.State)))
	if issue.VerdictReason != "" {
		fmt.Fprintf(w, "    %s\n", indent(issue.VerdictReason))
	}
}
