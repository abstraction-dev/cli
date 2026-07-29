package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The transcript is styled, so a selection has to slice by display column
// rather than by byte, and must not cut an escape sequence in half.
func TestSelectedTextSlicesStyledLines(t *testing.T) {
	lines := []string{
		"plain first line",
		"\x1b[1mbold\x1b[0m and \x1b[31mred\x1b[0m",
		"third line",
	}

	cases := []struct {
		name string
		sel  selection
		want string
	}{
		{
			name: "within one line",
			sel:  selection{anchor: selPoint{0, 6}, focus: selPoint{0, 11}, active: true},
			want: "first",
		},
		{
			name: "across styled text keeps the characters, drops the styling",
			sel:  selection{anchor: selPoint{1, 0}, focus: selPoint{1, 12}, active: true},
			want: "bold and red",
		},
		{
			name: "spanning lines takes the tail, the middle whole, and the head",
			sel:  selection{anchor: selPoint{0, 6}, focus: selPoint{2, 5}, active: true},
			want: "first line\nbold and red\nthird",
		},
		{
			name: "made backwards, read forwards",
			sel:  selection{anchor: selPoint{2, 5}, focus: selPoint{0, 6}, active: true},
			want: "first line\nbold and red\nthird",
		},
		{
			name: "inactive selects nothing",
			sel:  selection{anchor: selPoint{0, 0}, focus: selPoint{2, 5}},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := selectedText(lines, c.sel); got != c.want {
				t.Fatalf("selectedText = %q, want %q", got, c.want)
			}
		})
	}
}

// Copied lines lose their trailing padding, and blank lines inside a selection
// survive so the shape of what was on screen is preserved.
func TestSelectedTextTrimsPaddingAndKeepsBlanks(t *testing.T) {
	lines := []string{"one   ", "", "two   "}
	sel := selection{anchor: selPoint{0, 0}, focus: selPoint{2, 6}, active: true}

	if got, want := selectedText(lines, sel), "one\n\ntwo"; got != want {
		t.Fatalf("selectedText = %q, want %q", got, want)
	}
}

// Painting marks exactly the selected columns and leaves the rest of the line
// alone, including the styling outside the selection.
func TestPaintSelectionMarksOnlyTheSelectedColumns(t *testing.T) {
	lines := []string{"abcdefghij"}
	sel := selection{anchor: selPoint{0, 3}, focus: selPoint{0, 6}, active: true}

	got := paintSelection(lines, sel)[0]

	// The visible text is unchanged; only its styling differs.
	if plain := ansi.Strip(got); plain != "abcdefghij" {
		t.Fatalf("painting changed the text: %q", plain)
	}
	if !strings.Contains(got, selectedStyle.Render("def")) {
		t.Fatalf("selected columns not marked:\n%q", got)
	}
	if strings.Contains(ansi.Strip(got[:strings.Index(got, "def")]), "def") {
		t.Fatalf("marked the wrong columns:\n%q", got)
	}
}

// An inactive selection must leave the transcript exactly as it was — this is
// the common case, repainted on every frame while a turn streams.
func TestPaintSelectionInactiveIsUntouched(t *testing.T) {
	lines := []string{"one", "\x1b[1mtwo\x1b[0m"}

	got := paintSelection(lines, selection{})

	if len(got) != len(lines) {
		t.Fatalf("line count changed: %d want %d", len(got), len(lines))
	}
	for i := range lines {
		if got[i] != lines[i] {
			t.Fatalf("line %d changed: %q want %q", i, got[i], lines[i])
		}
	}
}

// A selection that runs past the end of the transcript (dragging below the last
// line) must not panic or invent content.
func TestSelectionOutOfRangeIsClamped(t *testing.T) {
	lines := []string{"only line"}
	sel := selection{anchor: selPoint{0, 0}, focus: selPoint{9, 40}, active: true}

	if got, want := selectedText(lines, sel), "only line"; got != want {
		t.Fatalf("selectedText = %q, want %q", got, want)
	}
	if got := paintSelection(lines, sel); len(got) != 1 {
		t.Fatalf("expected one line back, got %d", len(got))
	}
}

// span decides what each line contributes: the first line from the anchor, the
// last up to the focus, and everything between end to end.
func TestSelectionSpanPerLine(t *testing.T) {
	sel := selection{anchor: selPoint{1, 4}, focus: selPoint{3, 2}, active: true}

	cases := []struct {
		line, width int
		from, to    int
		ok          bool
	}{
		{line: 0, width: 10, ok: false},                 // above the selection
		{line: 1, width: 10, from: 4, to: 10, ok: true}, // first line: anchor to end
		{line: 2, width: 10, from: 0, to: 10, ok: true}, // middle: whole line
		{line: 3, width: 10, from: 0, to: 2, ok: true},  // last line: up to focus
		{line: 4, width: 10, ok: false},                 // below the selection
		{line: 2, width: 0, ok: false},                  // blank line has nothing to mark
	}

	for _, c := range cases {
		from, to, ok := sel.span(c.line, c.width)
		if ok != c.ok || (ok && (from != c.from || to != c.to)) {
			t.Errorf("span(line=%d,width=%d) = %d,%d,%v want %d,%d,%v",
				c.line, c.width, from, to, ok, c.from, c.to, c.ok)
		}
	}
}
