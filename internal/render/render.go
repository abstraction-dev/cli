// Package render handles terminal output: agent answer text to stdout,
// diagnostics/prompts/status to stderr, with optional ANSI color.
package render

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// defaultMarkdownWidth is used when no real terminal width is available (e.g.
// output is piped, not a tty).
const defaultMarkdownWidth = 100

// Markdown renders markdown to ANSI-styled terminal text, matching the terminal
// style. Safe for one-shot (immediate-mode) output where no full-screen program
// owns the terminal. On any failure it falls back to raw markdown.
//
// glamour renders in true color unconditionally, so the result goes through
// lipgloss to be downsampled to what stdout actually supports.
func Markdown(md string, width int) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(styleFor(hasDarkBackground())),
		glamour.WithWordWrap(clampWidth(width)),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return lipgloss.Sprint(out)
}

// hasDarkBackground reports whether the terminal has a dark background by
// querying it. This is blocking I/O on the terminal, so it must not be called
// while a bubbletea program owns it — a program listens for
// tea.BackgroundColorMsg instead (see the REPL).
func hasDarkBackground() bool {
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

// MDRenderer is a reusable markdown→ANSI renderer with a FIXED style. Unlike
// Markdown it performs no terminal queries at render time, so it is safe to use
// repeatedly inside a full-screen program (avoids OSC responses leaking into
// input). Rebuild via Resize when the terminal width changes.
type MDRenderer struct {
	style string
	width int
	tr    *glamour.TermRenderer
}

// NewMDRenderer builds a renderer for the given background and width.
func NewMDRenderer(dark bool, width int) *MDRenderer {
	m := &MDRenderer{style: styleFor(dark)}
	m.Resize(width)
	return m
}

// Resize rebuilds the renderer for a new width (same fixed style, no queries).
func (m *MDRenderer) Resize(width int) {
	m.width = width
	tr, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.style), glamour.WithWordWrap(clampWidth(width)))
	if err == nil {
		m.tr = tr
	}
}

// SetDark switches to the light or dark palette, rebuilding the renderer at its
// current width. A full-screen program that only learns the terminal background
// after it starts (via tea.BackgroundColorMsg) uses this to correct its guess.
func (m *MDRenderer) SetDark(dark bool) {
	if style := styleFor(dark); style != m.style {
		m.style = style
		m.Resize(m.width)
	}
}

func styleFor(dark bool) string {
	if dark {
		return styles.DarkStyle
	}
	return styles.LightStyle
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
	if width <= 0 {
		return defaultMarkdownWidth
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
