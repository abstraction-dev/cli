package cli

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestLoaderFrames checks the embedded asset parses into uniformly sized
// frames, which is what lets the animation be centred without jitter.
func TestLoaderFrames(t *testing.T) {
	if len(loaderFrames) < 2 {
		t.Fatalf("frames parsed: %d", len(loaderFrames))
	}
	if loaderWidth == 0 || loaderHeight == 0 {
		t.Fatalf("frame size: %dx%d", loaderWidth, loaderHeight)
	}
	for i, f := range loaderFrames {
		if w, h := lipgloss.Width(f), lipgloss.Height(f); w != loaderWidth || h != loaderHeight {
			t.Fatalf("frame %d is %dx%d, want %dx%d", i, w, h, loaderWidth, loaderHeight)
		}
		if strings.Contains(f, "---") {
			t.Fatalf("frame %d kept a separator:\n%s", i, f)
		}
	}
}

// TestLoaderViewCentered checks the animation lands in the middle of the box on
// both axes, at any window size, and that the box is filled exactly.
func TestLoaderViewCentered(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{80, 20}, {120, 40}, {31, 18}, {loaderWidth, loaderHeight + 2}} {
		m := &replModel{turnStarted: time.Now(), loaderIdx: 7}
		out, ok := m.loaderView(tc.w, tc.h)
		if !ok {
			t.Fatalf("%dx%d: loader declined a box that fits", tc.w, tc.h)
		}
		lines := strings.Split(out, "\n")
		if len(lines) != tc.h {
			t.Fatalf("%dx%d: got %d lines", tc.w, tc.h, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > tc.w {
				t.Fatalf("%dx%d: line %d overflows the box at width %d", tc.w, tc.h, i, w)
			}
		}

		// Horizontal: everything drawn must sit inside the centred slot, so
		// the columns either side of it are blank.
		pad := (tc.w - loaderWidth) / 2
		first, last := -1, -1
		for i, l := range lines {
			// Blank rows come back unpadded when the box exactly fits, so pad
			// to the box before slicing the slot out of them.
			r := []rune(ansi.Strip(l))
			for len(r) < tc.w {
				r = append(r, ' ')
			}
			outside := string(r[:pad]) + string(r[pad+loaderWidth:])
			if strings.TrimSpace(outside) != "" {
				t.Fatalf("%dx%d: line %d spills outside the centred slot: %q", tc.w, tc.h, i, outside)
			}
			if strings.TrimSpace(string(r)) != "" {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first < 0 {
			t.Fatalf("%dx%d: nothing drawn", tc.w, tc.h)
		}

		// Vertical: discounting the frame's own blank top rows, the gap above
		// the block matches the gap below it to within the odd row that cannot
		// be split evenly. The block's last row is the status, never blank.
		above := first - blankTopRows(loaderFrames[7%len(loaderFrames)])
		below := tc.h - 1 - last
		if diff := above - below; diff > 1 || diff < -1 {
			t.Fatalf("%dx%d: off-centre vertically, %d above vs %d below", tc.w, tc.h, above, below)
		}
	}
}

// TestLoaderViewTooSmall checks the animation is skipped rather than clipped
// when the window cannot hold it.
func TestLoaderViewTooSmall(t *testing.T) {
	m := &replModel{turnStarted: time.Now()}
	for _, tc := range []struct{ w, h int }{{loaderWidth - 1, 40}, {80, loaderHeight + 1}, {0, 0}} {
		if _, ok := m.loaderView(tc.w, tc.h); ok {
			t.Fatalf("%dx%d: loader accepted a box too small for it", tc.w, tc.h)
		}
	}
}

// TestLoadingWindow checks the animation covers exactly the wait for the first
// token: it starts with the turn and stops once answer text exists.
func TestLoadingWindow(t *testing.T) {
	m := &replModel{}
	if m.loading() {
		t.Fatal("loading while idle")
	}
	m.streaming = true
	if !m.loading() {
		t.Fatal("not loading at the start of a turn")
	}
	m.answer.WriteString("  \n ") // whitespace is not yet an answer
	if !m.loading() {
		t.Fatal("whitespace ended the loading window")
	}
	m.answer.WriteString("Auth is handled by middleware.")
	if m.loading() {
		t.Fatal("still loading after the first token")
	}
	m.streaming = false
	if m.loading() {
		t.Fatal("loading after the turn finished")
	}
}

// blankTopRows counts the all-space rows a frame opens with. They are part of
// the frame's fixed height, so centring puts them above the visible artwork.
func blankTopRows(frame string) int {
	n := 0
	for _, l := range strings.Split(frame, "\n") {
		if strings.TrimSpace(l) != "" {
			break
		}
		n++
	}
	return n
}

func TestStatusText(t *testing.T) {
	m := &replModel{turnStarted: time.Now().Add(-3 * time.Second)}
	if got := m.statusText(); got != "working… (3s)" {
		t.Fatalf("default status: %q", got)
	}
	m.status = "searching"
	if got := m.statusText(); got != "searching (3s)" {
		t.Fatalf("status: %q", got)
	}
}
