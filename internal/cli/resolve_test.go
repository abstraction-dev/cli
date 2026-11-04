package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abstraction-dev/cli/internal/apiclient"
)

func TestCheckRuling(t *testing.T) {
	for _, tc := range []struct {
		target string
		ruling apiclient.Ruling
		id     string
		bad    string
	}{
		{target: "#4211", ruling: apiclient.Ruling{Verdict: "fixed"}, id: "4211"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "open"}, id: "4211"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "invalid", Reason: "the nil is checked by the caller"}, id: "4211"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "duplicate", DuplicateOf: "#4200"}, id: "4211"},
		{target: "abc", ruling: apiclient.Ruling{Verdict: "fixed"}, bad: "not a finding id"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "confirmed"}, bad: "--as takes one of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "wont_fix", Reason: "  "}, bad: "needs a --reason"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "invalid"}, bad: "needs a --reason"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "duplicate"}, bad: "--duplicate-of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "fixed", DuplicateOf: "4200"}, bad: "--duplicate-of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "duplicate", DuplicateOf: "x"}, bad: "not a finding id"},
	} {
		ruling := tc.ruling
		id, err := checkRuling(tc.target, &ruling)
		switch {
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%s %+v: got %v, want %q", tc.target, tc.ruling, err, tc.bad)
		case tc.bad == "" && (err != nil || id != tc.id):
			t.Errorf("%s %+v: got %q %v", tc.target, tc.ruling, id, err)
		}
	}

	ruling := apiclient.Ruling{Verdict: "duplicate", DuplicateOf: "#4200"}
	if _, err := checkRuling("4211", &ruling); err != nil || ruling.DuplicateOf != "4200" {
		t.Fatalf("original %q %v", ruling.DuplicateOf, err)
	}
}

// The branch's newest review wins, whether its PR's or a pre-review, and a
// PR of another repository or branch never does.
func TestBranchReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workspaces/ws/pr-reviews":
			if r.URL.Query().Get("q") != "feature" {
				w.Write([]byte(`{"reviews":[]}`))
				return
			}
			w.Write([]byte(`{"reviews":[
				{"id":"90","prUrl":"https://github.com/acme/other/pull/3","prHeadRef":"feature","reportType":"PR"},
				{"id":"80","prUrl":"https://github.com/acme/api/pull/2","prHeadRef":"feature-two","reportType":"PR"},
				{"id":"41","prUrl":"https://github.com/Acme/API/pull/1","prHeadRef":"feature","reportType":"PR"}]}`))
		case "/api/workspaces/ws/branch-reviews":
			w.Write([]byte(`{"reviews":[{"id":"12","prHeadRef":"feature","reportType":"BRANCH"},{"id":"95","prHeadRef":"other","reportType":"BRANCH"}]}`))
		}
	}))
	defer server.Close()
	client := apiclient.New(server.URL, "k")

	if got, err := branchReview(context.Background(), client, "ws", "acme/api", "feature"); err != nil || got != "41" {
		t.Fatalf("got %q %v, want the PR's review", got, err)
	}
	if got, err := branchReview(context.Background(), client, "ws", "acme/api", "other"); err != nil || got != "95" {
		t.Fatalf("got %q %v, want the pre-review", got, err)
	}
	if _, err := branchReview(context.Background(), client, "ws", "acme/api", "nothing"); err == nil || !strings.Contains(err.Error(), "--report") {
		t.Fatalf("got %v, want a pointer to --report", err)
	}
}

func TestRulingError(t *testing.T) {
	for status, want := range map[int]string{
		http.StatusForbidden:  "you need to be an editor or admin of the workspace",
		http.StatusNotFound:   "review 41 has no finding #4211; pass --report <id> if it is in another review",
		http.StatusBadRequest: "a duplicate names the issue it duplicates (HTTP 400)",
	} {
		err := rulingError(&apiclient.APIError{Status: status, Message: "a duplicate names the issue it duplicates"}, "41", "4211")
		if err.Error() != want {
			t.Errorf("%d: got %q, want %q", status, err, want)
		}
	}
	if err := rulingError(errors.New("dial"), "41", "4211"); err.Error() != "dial" {
		t.Errorf("got %q", err)
	}
}

func TestPrintRuling(t *testing.T) {
	var out bytes.Buffer
	printRuling(&out, "4211", apiclient.Ruling{Verdict: "invalid"}, &apiclient.ReviewIssue{
		CommentID: "4211", Title: "nil deref", State: "RESOLVED", Verdict: "invalid", VerdictBy: "octocat", VerdictReason: "checked by\nthe caller",
	})
	if want := "#4211 nil deref\n  invalid by octocat · resolved\n    checked by\n    the caller\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}

	out.Reset()
	printRuling(&out, "4211", apiclient.Ruling{Verdict: "fixed"}, nil)
	if out.String() != "#4211 fixed\n" {
		t.Fatalf("got %q", out.String())
	}
}

// Pre-reviews name no repository, so pre-reviews of the branch in two
// repositories are left for --report to choose between.
func TestBranchReviewAmbiguousPreReviews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workspaces/ws/pr-reviews":
			w.Write([]byte(`{"reviews":[]}`))
		case "/api/workspaces/ws/branch-reviews":
			w.Write([]byte(`{"reviews":[{"id":"12","prHeadRef":"feature","reportType":"BRANCH"},{"id":"13","prHeadRef":"feature","reportType":"BRANCH"}]}`))
		}
	}))
	defer server.Close()

	got, err := branchReview(context.Background(), apiclient.New(server.URL, "k"), "ws", "acme/api", "feature")
	if err == nil || !strings.Contains(err.Error(), "--report") {
		t.Fatalf("got %q %v, want a pointer to --report", got, err)
	}
}
