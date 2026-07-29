package cli

// Mouse-driven text selection over the transcript.
//
// The terminal would normally draw a selection for us, but it stops doing so
// the moment a program turns on mouse reporting — and mouse reporting is the
// only way to receive wheel events. A terminal hands the mouse over wholesale;
// there is no mode that reports the wheel while leaving drags to the terminal
// (?1000, ?1002 and ?1003 all suppress it). So a program that wants both has to
// model the selection itself, paint it, and put it on the clipboard over OSC 52
// — which is what this file does.

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// selPoint is a position in the transcript's rendered lines: an index into the
// whole content rather than the visible window, so a selection survives
// scrolling, and a column measured in display cells.
type selPoint struct {
	line int
	col  int
}

// before reports whether p comes earlier in the transcript than q.
func (p selPoint) before(q selPoint) bool {
	if p.line != q.line {
		return p.line < q.line
	}
	return p.col < q.col
}

type selection struct {
	anchor   selPoint // where the drag started
	focus    selPoint // where the pointer has reached
	active   bool     // the drag covered some text
	dragging bool     // the button is still down
}

// bounds returns the selection's ends in reading order, which is not the order
// they were made in when the drag went up the screen.
func (s selection) bounds() (start, end selPoint) {
	if s.focus.before(s.anchor) {
		return s.focus, s.anchor
	}
	return s.anchor, s.focus
}

// span returns the half-open column range selected on a line of the given
// width, and whether any of that line is selected at all. Lines in the middle
// of a multi-line selection are covered end to end.
func (s selection) span(line, width int) (from, to int, ok bool) {
	start, end := s.bounds()
	if !s.active || line < start.line || line > end.line {
		return 0, 0, false
	}
	from, to = 0, width
	if line == start.line {
		from = start.col
	}
	if line == end.line {
		to = end.col
	}
	if to > width {
		to = width
	}
	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}

// paintSelection returns lines with the selected range highlighted. The
// original styling inside the selection is dropped rather than combined with
// the highlight, so the marked text reads clearly whatever it was coloured
// before — the same trade a terminal's own selection makes.
func paintSelection(lines []string, s selection) []string {
	if !s.active {
		return lines
	}
	start, end := s.bounds()

	out := make([]string, len(lines))
	copy(out, lines)
	for i := max(start.line, 0); i <= end.line && i < len(lines); i++ {
		line := lines[i]
		width := ansi.StringWidth(line)
		from, to, ok := s.span(i, width)
		if !ok {
			continue
		}
		marked := selectedStyle.Render(ansi.Strip(ansi.Cut(line, from, to)))
		out[i] = ansi.Cut(line, 0, from) + marked + ansi.Cut(line, to, width)
	}
	return out
}

// selectedText is the plain text of the selection, ready for the clipboard:
// styling stripped, trailing padding off each line, and blank lines kept so the
// shape of what was on screen survives the copy.
func selectedText(lines []string, s selection) string {
	if !s.active {
		return ""
	}
	start, end := s.bounds()

	var picked []string
	for i := max(start.line, 0); i <= end.line && i < len(lines); i++ {
		line := lines[i]
		width := ansi.StringWidth(line)
		from, to := 0, width
		if i == start.line {
			from = start.col
		}
		if i == end.line {
			to = end.col
		}
		if to > width {
			to = width
		}
		if from >= to {
			picked = append(picked, "")
			continue
		}
		picked = append(picked, strings.TrimRight(ansi.Strip(ansi.Cut(line, from, to)), " "))
	}
	return strings.Join(picked, "\n")
}
