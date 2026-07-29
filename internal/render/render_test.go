package render

import (
	"strings"
	"testing"
)

func TestMarkdownRenders(t *testing.T) {
	out := Markdown("some words and a [link](path/to/file.go)", 80)
	if out == "" {
		t.Fatal("expected rendered output, got empty")
	}
	// Links are transformed regardless of color profile (tests run without a
	// TTY, so the colors are downsampled away to plain text). ANSI styling
	// itself is exercised in the live terminal, not here.
	if strings.Contains(out, "](path/to/file.go)") {
		t.Fatalf("markdown link syntax not rendered: %q", out)
	}
	if !strings.Contains(out, "path/to/file.go") {
		t.Fatalf("link target dropped: %q", out)
	}
}

func TestMarkdownBlankIsEmpty(t *testing.T) {
	if got := Markdown("   \n  ", 80); got != "" {
		t.Fatalf("blank input should render empty, got %q", got)
	}
}
