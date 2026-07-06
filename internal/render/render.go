// Package render handles terminal output: agent answer text to stdout,
// diagnostics/prompts/status to stderr, with optional ANSI color.
package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// maxMarkdownWidth caps word-wrap so long lines stay readable on wide terminals.
const maxMarkdownWidth = 100

// Markdown renders markdown to ANSI-styled terminal text, auto-detecting the
// terminal style. Safe for one-shot (immediate-mode) output where no full-screen
// program owns the terminal. On any failure it falls back to raw markdown.
func Markdown(md string, width int) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(clampWidth(width)))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}

// HasDarkBackground reports whether the terminal has a dark background, using
// lipgloss's cached detection. bubbletea's package init() warms this cache with
// a single query before any program acquires the terminal, so calling it here
// does NOT issue a fresh terminal query — avoiding the classic bug where the
// query's response leaks into the TUI's input.
func HasDarkBackground() bool {
	return lipgloss.HasDarkBackground()
}

// MDRenderer is a reusable markdown→ANSI renderer with a FIXED style. Unlike
// Markdown it performs no terminal queries at render time, so it is safe to use
// repeatedly inside a full-screen program (avoids OSC responses leaking into
// input). Rebuild via Resize when the terminal width changes.
type MDRenderer struct {
	style string
	tr    *glamour.TermRenderer
}

// NewMDRenderer builds a renderer for the given background and width.
func NewMDRenderer(dark bool, width int) *MDRenderer {
	style := "light"
	if dark {
		style = "dark"
	}
	m := &MDRenderer{style: style}
	m.Resize(width)
	return m
}

// Resize rebuilds the renderer for a new width (same fixed style, no queries).
func (m *MDRenderer) Resize(width int) {
	tr, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.style), glamour.WithWordWrap(clampWidth(width)))
	if err == nil {
		m.tr = tr
	}
}

// Render renders md, falling back to raw text on any failure.
func (m *MDRenderer) Render(md string) string {
	if m.tr == nil || strings.TrimSpace(md) == "" {
		return md
	}
	out, err := m.tr.Render(md)
	if err != nil {
		return md
	}
	return out
}

func clampWidth(width int) int {
	if width <= 0 || width > maxMarkdownWidth {
		return maxMarkdownWidth
	}
	return width
}

// TermWidth returns the column count of w's terminal, or 0 if it isn't one.
func TermWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return width
}

// ANSI escape codes (a minimal subset, kept local so the CLI is a standalone
// module with no monorepo imports).
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
)

// Renderer writes agent output to Out and everything else (prompts, status,
// errors) to Err. Color is applied only when enabled.
type Renderer struct {
	Out   io.Writer
	Err   io.Writer
	Color bool
}

// New builds a Renderer. When color is nil, it auto-detects from whether out is
// a terminal; a non-nil color forces the setting.
func New(out, err io.Writer, color *bool) *Renderer {
	enabled := IsTerminal(out)
	if color != nil {
		enabled = *color
	}
	return &Renderer{Out: out, Err: err, Color: enabled}
}

// IsTerminal reports whether w is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Output writes agent answer text to stdout verbatim — the backend already
// formats it for a terminal (file-path citations + repo-root banner).
func (r *Renderer) Output(s string) { fmt.Fprint(r.Out, s) }

// Status prints a transient progress line to stderr (dim). It never touches
// stdout, so pipes stay clean.
func (r *Renderer) Status(s string) { fmt.Fprintln(r.Err, r.paint(ansiDim, "· "+s)) }

// Info prints an informational line to stderr.
func (r *Renderer) Info(s string) { fmt.Fprintln(r.Err, s) }

// Warn prints a warning line to stderr.
func (r *Renderer) Warn(s string) { fmt.Fprintln(r.Err, r.paint(ansiYellow, s)) }

// Error prints a plain (non-format) error line to stderr.
func (r *Renderer) Error(s string) { fmt.Fprintln(r.Err, r.paint(ansiRed, s)) }

// Errorf prints a formatted error line to stderr.
func (r *Renderer) Errorf(format string, a ...any) {
	fmt.Fprintln(r.Err, r.paint(ansiRed, fmt.Sprintf(format, a...)))
}

// Prompt returns the interactive REPL prompt string, colored when enabled.
func (r *Renderer) Prompt(label string) string { return r.paint(ansiBold+ansiCyan, label) }

func (r *Renderer) paint(code, s string) string {
	if !r.Color {
		return s
	}
	return code + s + ansiReset
}
