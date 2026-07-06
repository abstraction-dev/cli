package cli

import (
	"context"
	"errors"
	"testing"

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
