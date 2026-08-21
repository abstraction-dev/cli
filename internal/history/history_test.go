package history

import (
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T, limit int) *Store {
	t.Helper()
	return At(filepath.Join(t.TempDir(), "history.json"), limit)
}

func TestAppendTrimsToLimit(t *testing.T) {
	s := testStore(t, 2)
	for _, q := range []string{"first", "second", "third"} {
		if err := s.Append(Entry{Question: q, Answer: "a"}); err != nil {
			t.Fatalf("append %s: %v", q, err)
		}
	}

	got, err := s.Recent(0)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("kept %d entries, want 2", len(got))
	}

	// Recent is newest-first, so the trimmed-away "first" must be gone.
	if got[0].Question != "third" || got[1].Question != "second" {
		t.Fatalf("kept %q, %q; want third, second", got[0].Question, got[1].Question)
	}
}

func TestAppendSkipsBlankAndDisabled(t *testing.T) {
	s := testStore(t, 5)
	if err := s.Append(Entry{Question: "  ", Answer: "a"}); err != nil {
		t.Fatalf("blank question: %v", err)
	}

	if err := s.Append(Entry{Question: "q", Answer: ""}); err != nil {
		t.Fatalf("blank answer: %v", err)
	}

	got, err := s.All()
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("stored %d blank entries, want 0", len(got))
	}

	off := testStore(t, 0)
	if off.Enabled() {
		t.Fatal("zero limit should disable the store")
	}

	if err := off.Append(Entry{Question: "q", Answer: "a"}); err != nil {
		t.Fatalf("disabled append: %v", err)
	}
}

func TestAppendStampsAskedAt(t *testing.T) {
	s := testStore(t, 5)
	if err := s.Append(Entry{Question: "q", Answer: "a"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := s.Recent(1)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}

	if got[0].AskedAt.IsZero() {
		t.Fatal("AskedAt was not stamped")
	}

	if d := time.Since(got[0].AskedAt); d > time.Minute {
		t.Fatalf("AskedAt is %v old, want ~now", d)
	}
}

func TestAllOnMissingFileIsEmpty(t *testing.T) {
	got, err := testStore(t, 5).All()
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got %d entries from a missing file", len(got))
	}
}

func TestClearRemovesHistory(t *testing.T) {
	s := testStore(t, 5)
	if err := s.Append(Entry{Question: "q", Answer: "a"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := s.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}

	// A second clear must stay a no-op: the file is already gone.
	if err := s.Clear(); err != nil {
		t.Fatalf("second clear: %v", err)
	}

	got, err := s.All()
	if err != nil {
		t.Fatalf("all: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got %d entries after clear", len(got))
	}
}

func TestHeadlineCollapsesAndTruncates(t *testing.T) {
	e := Entry{Question: "how   does\nauth\twork"}
	if got := e.Headline(0); got != "how does auth work" {
		t.Fatalf("Headline(0) = %q", got)
	}

	if got := e.Headline(8); got != "how doe…" {
		t.Fatalf("Headline(8) = %q, want %q", got, "how doe…")
	}
}
