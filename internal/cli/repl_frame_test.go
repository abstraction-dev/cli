package cli

import (
	"strings"
	"testing"

	"github.com/abstraction-dev/cli/internal/render"
	"github.com/abstraction-dev/cli/internal/transport"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// TestREPLFrame drives the model the way the program does — a resize, a
// background report, keystrokes, a wheel event — and checks each reaches the
// right part of the frame.
func TestREPLFrame(t *testing.T) {
	ti := textarea.New()
	ti.Prompt = inputPrompt
	ti.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return inputPrompt
		}
		return ""
	})
	ti.ShowLineNumbers = false
	ti.SetHeight(inputHeight)
	ti.Focus()

	m := &replModel{
		env:   &appEnv{workspace: "ws-abcdef123"},
		sub:   make(chan tea.Msg, 8),
		md:    render.NewMDRenderer(true, 0),
		input: ti,
	}
	m.applyBackground(true)
	m.entries = []transcriptEntry{{entrySystem, "banner"}}

	if v := m.View(); !v.AltScreen || v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("view flags: alt=%v mouse=%v", v.AltScreen, v.MouseMode)
	}

	var mm tea.Model = m
	mm, _ = mm.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mm, _ = mm.Update(tea.BackgroundColorMsg{})

	// Type "hi", check it lands in the input and renders with the prompt.
	for _, r := range "hi" {
		mm, _ = mm.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := m.input.Value(); got != "hi" {
		t.Fatalf("input value: %q", got)
	}
	out := m.View().Content
	if !strings.Contains(out, "hi") || !strings.Contains(out, "banner") {
		t.Fatalf("frame missing input or transcript:\n%s", out)
	}

	// Markdown answers render into the transcript.
	m.entries = append(m.entries, transcriptEntry{entryAnswer, "# Title\n\nsome **bold** text\n"})
	m.refresh()
	if out := m.View().Content; !strings.Contains(out, "Title") {
		t.Fatalf("markdown not rendered:\n%s", out)
	}

	// ctrl+c clears the input when idle; ctrl+d then quits.
	mm, _ = mm.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.input.Value() != "" {
		t.Fatalf("ctrl+c should clear the input, got %q", m.input.Value())
	}
	if _, cmd := mm.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}); cmd == nil {
		t.Fatal("ctrl+d on an empty input should quit")
	}

	// Pickers: enter picker mode, arrow down, render, escape.
	m.openChatPicker(chatPickerLoadedMsg{items: []transport.Chat{{Slug: "a", Title: "one"}, {Slug: "b", Title: "two"}}})
	mm, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.chatIdx != 1 {
		t.Fatalf("picker down: idx=%d", m.chatIdx)
	}
	if out := m.View().Content; !strings.Contains(out, "two") || !strings.Contains(out, "selecting conversation…") {
		t.Fatalf("picker frame:\n%s", out)
	}
	mm, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeNormal {
		t.Fatalf("esc should leave the picker, mode=%v", m.mode)
	}

	// Mouse wheel and pgup/pgdown reach the transcript viewport rather than the input.
	mm, _ = mm.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	mm, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.input.Value() != "" {
		t.Fatalf("scroll keys must not type into the input, got %q", m.input.Value())
	}
	_ = mm
}

// Dragging over the transcript selects text and copies it. The terminal cannot
// do this for us while mouse reporting is on, so the whole path — press, drag,
// release, clipboard — belongs to the program.
func TestMouseDragSelectsAndCopies(t *testing.T) {
	m := &replModel{
		env:   &appEnv{workspace: "ws-abcdef123"},
		sub:   make(chan tea.Msg, 8),
		md:    render.NewMDRenderer(true, 0),
		input: textarea.New(),
	}
	m.applyBackground(true)
	m.resize(80, 24)
	m.entries = []transcriptEntry{{entrySystem, "selectable transcript line"}}
	m.refresh()

	var mm tea.Model = m

	// Press at column 0 of the first row, drag right, release.
	mm, _ = mm.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if !m.sel.dragging || m.sel.active {
		t.Fatalf("a press alone should start a drag but select nothing: %+v", m.sel)
	}
	mm, _ = mm.Update(tea.MouseMotionMsg{X: 10, Y: 0, Button: tea.MouseLeft})
	if !m.sel.active {
		t.Fatal("moving the pointer should make the selection active")
	}
	if out := m.View().Content; !strings.Contains(out, selectedStyle.Render("selectable")) {
		t.Fatalf("selection not highlighted in the frame:\n%q", out)
	}

	_, cmd := mm.Update(tea.MouseReleaseMsg{X: 10, Y: 0, Button: tea.MouseLeft})
	if m.sel.dragging {
		t.Fatal("release should end the drag")
	}
	if cmd == nil {
		t.Fatal("release should hand the selection to the clipboard")
	}
	if !m.copied || !strings.Contains(m.View().Content, "copied the selection") {
		t.Fatalf("the copy should be reported in the hint line, copied=%v", m.copied)
	}

	// Esc dismisses it, since there is no terminal selection to click away from.
	mm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.sel.active || m.copied {
		t.Fatalf("esc should clear the selection: %+v copied=%v", m.sel, m.copied)
	}
}

// A bare click selects nothing and must not put anything on the clipboard.
func TestMouseClickWithoutDragCopiesNothing(t *testing.T) {
	m := &replModel{env: &appEnv{}, md: render.NewMDRenderer(true, 0), input: textarea.New()}
	m.applyBackground(true)
	m.resize(80, 24)
	m.entries = []transcriptEntry{{entrySystem, "a line"}}
	m.refresh()

	var mm tea.Model = m
	mm, _ = mm.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	_, cmd := mm.Update(tea.MouseReleaseMsg{X: 2, Y: 0, Button: tea.MouseLeft})

	if cmd != nil {
		t.Fatal("a click with no drag should not reach the clipboard")
	}
	if m.sel.active || m.copied {
		t.Fatalf("nothing should be selected: %+v copied=%v", m.sel, m.copied)
	}
}
