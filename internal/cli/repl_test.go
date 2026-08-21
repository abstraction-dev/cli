package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abstraction-dev/cli/internal/transport"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func newHistoryModel() *replModel {
	ti := textarea.New()
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
	ti := textarea.New()
	m := &replModel{input: ti}
	m.historyPrev() // no history — must not panic or change anything
	m.historyNext()
	if m.input.Value() != "" {
		t.Fatalf("expected empty input, got %q", m.input.Value())
	}
}

func TestMatchPRReview(t *testing.T) {
	reviews := []transport.PRReview{
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
	if !(transport.PRReview{Status: "COMPLETED"}).Ready() {
		t.Fatal("COMPLETED should be ready")
	}
	for _, s := range []string{"PENDING", "IN_PROGRESS", "FAILED", "CANCELLED", ""} {
		if (transport.PRReview{Status: s}).Ready() {
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

// exchange is one stored exchange as GET /api/chats/{slug} delivers it.
func exchange(raw string) transport.ChatMessage {
	return transport.ChatMessage{Messages: []byte(raw)}
}

func TestResumeConversationReplaysTranscript(t *testing.T) {
	m := &replModel{input: textarea.New(), sessionID: "fresh-uuid"}
	m.entries = []transcriptEntry{{entrySystem, "banner"}}

	m.resumeConversation(transport.ChatWithMessages{
		Chat: transport.Chat{Slug: "chat-1", Title: "Where is auth handled", Type: "DEFAULT"},
		Messages: []transport.ChatMessage{
			exchange(`[{"role":"user","content":[{"type":"text","text":"where is auth handled"}]},
			           {"role":"assistant","content":[{"type":"text","text":"In internal/auth."}]}]`),
			exchange(`[{"role":"user","content":[{"type":"text","text":"and the api keys"}]},
			           {"role":"tool","tool_call_id":"t1","content":[{"type":"text","text":"rows"}]},
			           {"role":"assistant","content":[{"type":"text","text":"Hashed with bcrypt."}]}]`),
		},
	})

	// The conversation's slug is the session id from here on — that is what makes the
	// next question continue this conversation rather than start one.
	if m.sessionID != "chat-1" {
		t.Fatalf("session id: got %q, want chat-1", m.sessionID)
	}
	if m.activePR != "" {
		t.Fatalf("a workspace conversation must not set a PR scope, got %q", m.activePR)
	}

	want := []transcriptEntry{
		{entryUser, "where is auth handled"},
		{entryAnswer, "In internal/auth."},
		{entryUser, "and the api keys"},
		{entryAnswer, "Hashed with bcrypt."},
		{entrySystem, "resumed conversation: Where is auth handled"},
	}
	if len(m.entries) != len(want) {
		t.Fatalf("expected %d entries, got %d: %+v", len(want), len(m.entries), m.entries)
	}
	for i := range want {
		if m.entries[i] != want[i] {
			t.Fatalf("entry %d: got %+v, want %+v", i, m.entries[i], want[i])
		}
	}

	// Replayed questions are recallable with ↑, like the ones typed this run.
	if len(m.history) != 2 || m.history[0] != "where is auth handled" || m.histIdx != 2 {
		t.Fatalf("input history: %+v idx=%d", m.history, m.histIdx)
	}
}

// A resumed PR conversation restores its scope, so the status bar reports what the
// answers are grounded in and /new keeps that grounding.
func TestResumeConversationRestoresPRScope(t *testing.T) {
	m := &replModel{input: textarea.New()}

	m.resumeConversation(transport.ChatWithMessages{
		Chat: transport.Chat{Slug: "chat-2", Title: "Review", Type: transport.ChatTypePR, DiffReportID: "42", PRNumber: "123"},
	})

	if m.activePR != "42" {
		t.Fatalf("pr scope: got %q, want the diff report id 42", m.activePR)
	}
	if last := m.entries[len(m.entries)-1]; last.text != "resumed conversation: #123 · Review" {
		t.Fatalf("resume note: got %q", last.text)
	}
}

// An exchange that can't be read costs its turns, not the resume.
func TestResumeConversationReportsUnreadableExchange(t *testing.T) {
	m := &replModel{input: textarea.New()}

	m.resumeConversation(transport.ChatWithMessages{
		Chat: transport.Chat{Slug: "chat-3", Title: "Broken"},
		Messages: []transport.ChatMessage{
			exchange(`not json`),
			exchange(`[{"role":"user","content":[{"type":"text","text":"still here"}]}]`),
		},
	})

	if m.entries[0] != (transcriptEntry{entryUser, "still here"}) {
		t.Fatalf("expected the readable turn to survive, got %+v", m.entries)
	}
	last := m.entries[len(m.entries)-1]
	if last.kind != entrySystem || !strings.Contains(last.text, "could not be replayed") {
		t.Fatalf("expected a note about the unreadable exchange, got %+v", last)
	}
}

// /new lets go of the conversation being held. Holding none is what makes the next
// question start one, which the backend then names.
func TestNewConversationLeavesCurrentConversation(t *testing.T) {
	m := &replModel{input: textarea.New(), sessionID: "chat-1"}
	m.entries = []transcriptEntry{{entryUser, "earlier question"}}

	m.newConversation()

	if m.sessionID != "" {
		t.Fatalf("expected no conversation to be held, got %q", m.sessionID)
	}
	if len(m.entries) != 0 {
		t.Fatalf("expected a cleared transcript, got %+v", m.entries)
	}
}

// The backend names a conversation on its first question and the REPL adopts that
// name, so every later question continues it instead of starting another.
func TestConversationMsgIsAdopted(t *testing.T) {
	m := &replModel{input: textarea.New(), sub: make(chan tea.Msg, 1)}

	if _, cmd := m.Update(conversationMsg("chat-9")); cmd == nil {
		t.Fatal("expected the sub channel to be re-armed")
	}
	if m.sessionID != "chat-9" {
		t.Fatalf("session id: got %q, want chat-9", m.sessionID)
	}

	// A later turn on the same conversation reports the same slug; adopting it again
	// must not disturb the transcript.
	m.entries = []transcriptEntry{{entryUser, "a question"}}
	m.Update(conversationMsg("chat-9"))

	if len(m.entries) != 1 || m.sessionID != "chat-9" {
		t.Fatalf("re-adopting changed state: entries=%+v session=%q", m.entries, m.sessionID)
	}
}

// The picker opens on the conversation being held, so the list says where you are.
func TestOpenChatPickerPreselectsActiveConversation(t *testing.T) {
	m := &replModel{input: textarea.New(), sessionID: "chat-2"}
	items := []transport.Chat{{Slug: "chat-1"}, {Slug: "chat-2"}, {Slug: "chat-3"}}

	m.openChatPicker(chatPickerLoadedMsg{items: items})

	if m.mode != modePickingChat {
		t.Fatalf("expected the chat picker mode, got %v", m.mode)
	}
	if m.chatIdx != 1 {
		t.Fatalf("expected the active conversation preselected, got index %d", m.chatIdx)
	}

	// A conversation started here matches nothing in the list, so it starts at the top.
	fresh := &replModel{input: textarea.New(), sessionID: "fresh-uuid"}
	fresh.openChatPicker(chatPickerLoadedMsg{items: items})
	if fresh.chatIdx != 0 {
		t.Fatalf("expected index 0 for an unlisted conversation, got %d", fresh.chatIdx)
	}
}

func TestOpenChatPickerEmptyAndError(t *testing.T) {
	empty := &replModel{input: textarea.New()}
	empty.openChatPicker(chatPickerLoadedMsg{})
	if empty.mode != modeNormal {
		t.Fatal("an empty list must not enter picker mode")
	}
	if empty.entries[0].kind != entrySystem {
		t.Fatalf("expected a system note, got %+v", empty.entries[0])
	}

	failed := &replModel{input: textarea.New()}
	failed.openChatPicker(chatPickerLoadedMsg{err: errors.New("boom")})
	if failed.mode != modeNormal {
		t.Fatal("a failed load must not enter picker mode")
	}
	if failed.entries[0].kind != entryError {
		t.Fatalf("expected an error entry, got %+v", failed.entries[0])
	}
}

func TestChatLabel(t *testing.T) {
	cases := []struct {
		name string
		chat transport.Chat
		want string
	}{
		{"title", transport.Chat{Slug: "0123456789ab", Title: "Where is auth handled"}, "Where is auth handled"},
		{"untitled falls back to the slug", transport.Chat{Slug: "0123456789ab"}, "01234567…"},
		{"pr chat is labelled by its pull request", transport.Chat{Title: "Review", Type: transport.ChatTypePR, PRNumber: "123"}, "#123 · Review"},
		{"pr chat without a number", transport.Chat{Title: "Review", Type: transport.ChatTypePR}, "Review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chatLabel(tc.chat); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChatWhen(t *testing.T) {
	if when := chatWhen(transport.Chat{CreatedAt: "2026-07-20T10:11:12Z"}); when == "" {
		t.Fatal("expected a formatted timestamp")
	}
	if got := chatWhen(transport.Chat{CreatedAt: "not a time"}); got != "" {
		t.Fatalf("expected an unparseable timestamp to be left out, got %q", got)
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
