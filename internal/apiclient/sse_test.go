package apiclient

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, raw string) []sseEvent {
	t.Helper()
	r := newSSEReader(io.NopCloser(strings.NewReader(raw)))
	var out []sseEvent
	for {
		ev, err := r.next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		out = append(out, ev)
	}
}

func TestSSEBasicEvents(t *testing.T) {
	raw := "event: output\ndata: {\"text\":\"hi\"}\n\n" +
		"event: stream_end\n\n"
	got := readAll(t, raw)
	want := []sseEvent{
		{Type: "output", Data: `{"text":"hi"}`},
		{Type: "stream_end"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A bare `event:` frame (no data) must be returned as its own event; its type
// must not bleed onto the following data-only frame.
func TestSSEDatalessTypeDoesNotBleed(t *testing.T) {
	raw := "event: status\n\n" +
		"data: {\"text\":\"answer\"}\n\n"
	got := readAll(t, raw)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(got), got)
	}
	if got[0].Type != "status" || got[0].Data != "" {
		t.Fatalf("event 0: got %+v, want a data-less status event", got[0])
	}
	if got[1].Type != "" || got[1].Data != `{"text":"answer"}` {
		t.Fatalf("event 1: got %+v, want an untyped data frame (no bled type)", got[1])
	}
}

// A stream ending on a bare typed frame with no trailing blank line still
// delivers that event rather than swallowing it as EOF.
func TestSSETrailingDatalessEvent(t *testing.T) {
	got := readAll(t, "event: stream_end\n")
	if len(got) != 1 || got[0].Type != "stream_end" {
		t.Fatalf("got %+v, want a single stream_end event", got)
	}
}

func TestSSECommentsAndKeepAlivesIgnored(t *testing.T) {
	raw := ": keep-alive\n" +
		"event: output\n" +
		"data: line1\n" +
		"data: line2\n" +
		"\n"
	got := readAll(t, raw)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	if got[0].Type != "output" || got[0].Data != "line1\nline2" {
		t.Fatalf("got %+v, want output with joined data lines", got[0])
	}
}
