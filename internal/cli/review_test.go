package cli

import (
	"bytes"
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/abstraction-dev/cli/internal/apiclient"
)

func TestGithubRepository(t *testing.T) {
	for remote, want := range map[string]string{
		"git@github.com:acme/api.git":          "acme/api",
		"https://github.com/acme/api":          "acme/api",
		"https://github.com/acme/api.git":      "acme/api",
		"ssh://git@github.com/acme/my.api.git": "acme/my.api",
		"https://gitlab.com/acme/api.git":      "",
	} {
		got, ok := githubRepository(remote)
		if got != want || ok != (want != "") {
			t.Errorf("%s: got %q %v, want %q", remote, got, ok, want)
		}
	}
}

func TestParseInterspersed(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	base := fs.String("base", "", "")
	got, err := parseInterspersed(fs, []string{"https://github.com/a/b/pull/1", "-json", "-base", "dev"})
	if err != nil || !slices.Equal(got, []string{"https://github.com/a/b/pull/1"}) || !*asJSON || *base != "dev" {
		t.Fatalf("got %v %v json=%v base=%q", got, err, *asJSON, *base)
	}
}

func step(name, state string, done, total int64) apiclient.ReviewStep {
	return apiclient.ReviewStep{Name: name, State: state, Done: &done, Total: &total}
}

// Off a terminal each change is one line, and a repeat prints nothing.
func TestProgressViewPrintsALinePerChange(t *testing.T) {
	var out bytes.Buffer
	view := newProgressView(&out, false)
	for _, progress := range []apiclient.ReviewProgress{
		{Headline: "Queued · waiting for a worker"},
		{Headline: "Queued · waiting for a worker"},
		{Headline: "Reviewing", Fraction: 0.5},
	} {
		view.update(progress)
	}
	if got := out.String(); got != "  0% Queued · waiting for a worker\n 50% Reviewing\n" {
		t.Fatalf("got %q", got)
	}
}

// On a terminal the step list is redrawn over the previous one.
func TestProgressViewRedrawsInPlace(t *testing.T) {
	var out bytes.Buffer
	view := newProgressView(&out, true)
	review := step("Review the changes", "running", 0, 1)
	review.Detail = "Reviewing"
	review.Substeps = []apiclient.ReviewStep{step("Reviewing functions", "running", 34, 77)}
	view.update(apiclient.ReviewProgress{Headline: "Reviewing", Steps: []apiclient.ReviewStep{
		{Name: "Queued", State: "completed", Detail: "Picked up by a worker"},
		review,
	}})
	want := "Reviewing  0%\n  ✓ Queued · Picked up by a worker\n  ● Review the changes · Reviewing\n      ● Reviewing functions (34/77)\n"
	if out.String() != want {
		t.Fatalf("first draw %q, want %q", out.String(), want)
	}
	out.Reset()
	view.update(apiclient.ReviewProgress{Headline: "Review finished", Fraction: 1})
	if !strings.HasPrefix(out.String(), "\x1b[4F\x1b[J") {
		t.Fatalf("redraw %q, want it to move up over the 4 lines drawn", out.String())
	}
}

func TestPrintFindings(t *testing.T) {
	var out bytes.Buffer
	printFindings(&out, apiclient.ReviewFinished{Status: "COMPLETED", Verdict: "FAIL", URL: "https://app/r/41"}, []apiclient.ReviewIssue{
		{Severity: "high", Title: "nil deref", Description: "Dereferences before the check.", FilePath: "a.go", StartLine: 3},
	})
	want := "Review completed · FAIL · 1 open finding(s)\nhttps://app/r/41\n\n  [high] a.go:3\n    nil deref\n    Dereferences before the check.\n"
	if out.String() != want {
		t.Fatalf("got %q", out.String())
	}
}

func TestMatchBranchReview(t *testing.T) {
	reviews := []apiclient.PRReview{
		{ID: "7", ReportType: apiclient.ReportTypeBranch, PRHeadRef: "feature", PRBaseRef: "main"},
		{ID: "8", PRURL: "https://github.com/a/b/pull/1", PRNumber: 1, PRHeadRef: "other"},
	}
	got := matchBranchReview(reviews, "feature")
	if got == nil || got.Scope() != "7" || prLabel(*got) != "feature → main" {
		t.Fatalf("got %+v", got)
	}
	if matchBranchReview(reviews, "other") != nil {
		t.Fatal("a PR review matched as a pre-review")
	}
}
