package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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
		{target: "abc", ruling: apiclient.Ruling{Verdict: "fixed"}, bad: "not an issue id"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "confirmed"}, bad: "--as takes one of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "wont_fix", Reason: "  "}, bad: "needs a --reason"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "invalid"}, bad: "needs a --reason"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "duplicate"}, bad: "--duplicate-of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "fixed", DuplicateOf: "4200"}, bad: "--duplicate-of"},
		{target: "4211", ruling: apiclient.Ruling{Verdict: "duplicate", DuplicateOf: "x"}, bad: "not an issue id"},
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

// The review holding the issue is picked among this repository's PR reviews
// and the caller's pre-reviews of the branch; another repository's PR review
// of a same-named branch is never read.
func TestIssueReview(t *testing.T) {
	var read []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/workspaces/ws/pr-reviews":
			w.Write([]byte(`{"reviews":[
				{"id":"90","prUrl":"https://github.com/acme/other/pull/3","prHeadRef":"feature","reportType":"PR"},
				{"id":"80","prUrl":"https://github.com/acme/api/pull/2","prHeadRef":"feature-two","reportType":"PR"},
				{"id":"41","prUrl":"https://github.com/Acme/API/pull/1","prHeadRef":"feature","reportType":"PR"}]}`))
		case "/api/workspaces/ws/branch-reviews":
			w.Write([]byte(`{"reviews":[{"id":"13","prHeadRef":"feature","reportType":"BRANCH"},{"id":"12","prHeadRef":"feature","reportType":"BRANCH"}]}`))
		case "/api/workspaces/ws/pr-reviews/41":
			read = append(read, "41")
			w.Write([]byte(`{"issues":[{"commentId":"4100"}]}`))
		case "/api/workspaces/ws/pr-reviews/13":
			read = append(read, "13")
			w.Write([]byte(`{"issues":[{"commentId":"1300"}]}`))
		case "/api/workspaces/ws/pr-reviews/12":
			read = append(read, "12")
			w.Write([]byte(`{"issues":[{"commentId":"1200"}]}`))
		default:
			t.Errorf("read %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := apiclient.New(server.URL, "k")

	for issueID, want := range map[string]string{"4100": "41", "1300": "13", "1200": "12"} {
		if got, err := issueReview(context.Background(), client, "ws", "acme/api", "feature", issueID); err != nil || got != want {
			t.Errorf("issue %s: got %q %v, want %s", issueID, got, err, want)
		}
	}
	if slices.Contains(read, "90") {
		t.Errorf("read another repository's review: %v", read)
	}
	if _, err := issueReview(context.Background(), client, "ws", "acme/api", "feature", "9999"); err == nil || !strings.Contains(err.Error(), "has issue #9999") {
		t.Fatalf("got %v, want no review holding the issue", err)
	}
	if _, err := issueReview(context.Background(), client, "ws", "acme/api", "nothing", "4100"); err == nil || !strings.Contains(err.Error(), "no review of nothing") {
		t.Fatalf("got %v, want no review of the branch", err)
	}
}

func TestRulingError(t *testing.T) {
	for status, want := range map[int]string{
		http.StatusForbidden:  "you need to be an editor or admin of the workspace",
		http.StatusNotFound:   "review 41 has no issue #4211; pass --report <id> if it is in another review",
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
		IssueID: "4211", Title: "nil deref", State: "RESOLVED", Verdict: "invalid", VerdictBy: "octocat", VerdictReason: "checked by\nthe caller",
	})
	if want := "#4211 nil deref\n  invalid by octocat · resolved\n    checked by\n    the caller\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}

	out.Reset()
	printRuling(&out, "4211", apiclient.Ruling{Verdict: "open"}, &apiclient.ReviewIssue{IssueID: "4211", Title: "nil deref", State: "ACTIVE"})
	if want := "#4211 nil deref\n  open · active\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}

	out.Reset()
	printRuling(&out, "4211", apiclient.Ruling{Verdict: "fixed"}, nil)
	if out.String() != "#4211 fixed\n" {
		t.Fatalf("got %q", out.String())
	}
}
