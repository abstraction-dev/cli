package cli

import (
	"strings"
	"testing"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/render"

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
	m.openChatPicker(chatPickerLoadedMsg{items: []apiclient.Chat{{Slug: "a", Title: "one"}, {Slug: "b", Title: "two"}}})
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
