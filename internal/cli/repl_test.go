package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/charmbracelet/bubbles/textinput"
)

func newHistoryModel() *replModel {
	ti := textinput.New()
	return &replModel{
		input:   ti,
		history: []string{"first", "second", "third"},
		histIdx: 3, // == len(history): editing a fresh draft
	}
}

func TestHistoryNavigation(t *testing.T) {
	m := newHistoryModel()
	m.input.SetValue("draft")

	m.historyPrev()
	if got := m.input.Value(); got != "third" {
		t.Fatalf("prev 1: got %q, want third", got)
	}
	m.historyPrev()
	m.historyPrev()
	if got := m.input.Value(); got != "first" {
		t.Fatalf("prev 3: got %q, want first", got)
	}
	m.historyPrev() // clamp at oldest
	if got := m.input.Value(); got != "first" {
		t.Fatalf("prev clamp: got %q, want first", got)
	}

	m.historyNext()
	m.historyNext()
	if got := m.input.Value(); got != "third" {
		t.Fatalf("next to newest: got %q, want third", got)
	}
	m.historyNext() // back to the stashed draft
	if got := m.input.Value(); got != "draft" {
		t.Fatalf("draft restore: got %q, want draft", got)
	}
	m.historyNext() // clamp at draft
	if got := m.input.Value(); got != "draft" {
		t.Fatalf("next clamp: got %q, want draft", got)
	}
}

func TestHistoryNavigationEmpty(t *testing.T) {
	ti := textinput.New()
	m := &replModel{input: ti}
	m.historyPrev() // no history — must not panic or change anything
	m.historyNext()
	if m.input.Value() != "" {
		t.Fatalf("expected empty input, got %q", m.input.Value())
	}
}

func TestMatchPRReview(t *testing.T) {
	reviews := []apiclient.PRReview{
		{PRNumber: 12, PRURL: "https://github.com/acme/web/pull/12", Status: "COMPLETED"},
		{PRNumber: 34, PRURL: "https://github.com/acme/api/pull/34", Status: "IN_PROGRESS"},
	}

	cases := []struct {
		name string
		url  string
		want int // matched PR number, 0 == no match
	}{
		{"exact url", "https://github.com/acme/web/pull/12", 12},
		{"url trailing slash", "https://github.com/acme/web/pull/12/", 12},
		{"url http scheme", "http://github.com/acme/api/pull/34", 34},
		{"url uppercase host", "https://GitHub.com/acme/web/pull/12", 12},
		{"unknown url", "https://github.com/acme/web/pull/99", 0},
		{"bare number not accepted", "34", 0},
		{"garbage", "not-a-pr", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchPRReview(reviews, tc.url)
			switch {
			case tc.want == 0 && got != nil:
				t.Fatalf("url %q: expected no match, got #%d", tc.url, got.PRNumber)
			case tc.want != 0 && got == nil:
				t.Fatalf("url %q: expected #%d, got no match", tc.url, tc.want)
			case tc.want != 0 && got.PRNumber != tc.want:
				t.Fatalf("url %q: expected #%d, got #%d", tc.url, tc.want, got.PRNumber)
			}
		})
	}
}

func TestPRReviewReady(t *testing.T) {
	if !(apiclient.PRReview{Status: "COMPLETED"}).Ready() {
		t.Fatal("COMPLETED should be ready")
	}
	for _, s := range []string{"PENDING", "IN_PROGRESS", "FAILED", "CANCELLED", ""} {
		if (apiclient.PRReview{Status: s}).Ready() {
			t.Fatalf("status %q should not be ready", s)
		}
	}
}

func TestPRStatusLabel(t *testing.T) {
	cases := map[string]string{
		"IN_PROGRESS": "in progress",
		"COMPLETED":   "completed",
		"":            "pending",
	}
	for in, want := range cases {
		if got := prStatusLabel(in); got != want {
			t.Fatalf("prStatusLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCanceled(t *testing.T) {
	if !isCanceled(context.Canceled) {
		t.Fatal("context.Canceled should be canceled")
	}
	if !isCanceled(errors.New("Post ...: context canceled")) {
		t.Fatal("wrapped transport cancel should be canceled")
	}
	if isCanceled(nil) {
		t.Fatal("nil is not canceled")
	}
	if isCanceled(errors.New("boom")) {
		t.Fatal("generic error is not canceled")
	}
}
