package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/abstraction-dev/cli/assets"

	"charm.land/lipgloss/v2"
)

// loaderInterval is how often the loading animation advances a frame. The
// asset holds a full rotation, so this sets the rotation's speed: ~30fps.
const loaderInterval = 33 * time.Millisecond

// loaderStyle colours the animation with the same accent the prompt uses.
var loaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

// loaderFrames is the loading animation, one rotation of the Abstraction
// symbol, parsed once from the embedded asset.
var loaderFrames = parseLoaderFrames(assets.ASCIISpinner)

// loaderWidth and loaderHeight are the frame dimensions. Every frame is padded
// to the same size by the converter that produced the asset, so measuring the
// first one measures them all.
var loaderWidth, loaderHeight = frameSize(loaderFrames)

// loaderBand is how many transcript rows the animation takes over: the frame
// itself, a blank row, and the status line.
var loaderBand = loaderHeight + 2

// loaderMinTranscript is how many rows of transcript must survive underneath
// the animation for reserving the band to be worth it. Below that the window
// is too short to show both, and the animation gives way to the one-character
// spinner on the hint line.
const loaderMinTranscript = 3

// fallbackFrames is the one-character spinner shown wherever the band will not
// fit, so that a small window still gets a moving indicator rather than a
// static line. It is the spinner the REPL used before the ASCII animation.
var fallbackFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// writingStatus is what the status line reads once the answer is coming
// through, in place of the last thing the backend reported doing.
const writingStatus = "Writing response…"

// fallbackEvery is how many ticks each one-character frame is held for. The
// tick runs at 30fps to drive the ASCII animation, which would spin this one
// three times faster than it used to go, so it is slowed back to its original
// ~100ms a frame.
const fallbackEvery = 3

// parseLoaderFrames splits the asset into frames on its "---" separator lines.
func parseLoaderFrames(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	var frames []string
	for _, chunk := range strings.Split(raw, "\n---\n") {
		// Only newlines are trimmed: a frame's blank rows are runs of spaces
		// that carry its height, and dropping them would jitter the centring.
		if chunk = strings.Trim(chunk, "\n"); chunk != "" {
			frames = append(frames, chunk)
		}
	}
	return frames
}

func frameSize(frames []string) (w, h int) {
	if len(frames) == 0 {
		return 0, 0
	}
	return lipgloss.Width(frames[0]), lipgloss.Height(frames[0])
}

// loading reports whether the turn has yet to produce any answer text. That is
// the window the animation fills; once tokens arrive the transcript takes over
// and the status moves to the hint line.
func (m *replModel) loading() bool {
	return m.streaming && strings.TrimSpace(m.answer.String()) == ""
}

// loaderFits reports whether the window can hold the reserved block and still
// leave a usable amount of conversation above it. It gates the reservation as a
// whole, so a window too small for the animation reserves nothing and the
// answer streams the way it always did.
func (m *replModel) loaderFits() bool {
	return len(loaderFrames) > 0 &&
		m.width >= loaderWidth &&
		m.vp.Height() >= loaderBand+loaderMinTranscript
}

// loaderRows renders the current animation frame with the status beneath it,
// centred horizontally, as exactly loaderBand rows. That fixed height is what
// keeps the conversation still: the answer later streams into the same rows.
func (m *replModel) loaderRows() []string {
	frame := loaderFrames[m.loaderIdx%len(loaderFrames)]
	block := loaderStyle.Render(frame) +
		"\n\n" +
		lipgloss.PlaceHorizontal(loaderWidth, lipgloss.Center, faintStyle.Render(m.statusText()))
	return strings.Split(lipgloss.Place(m.width, loaderBand, lipgloss.Center, lipgloss.Center, block), "\n")
}

// fallbackLine renders the one-character spinner and the status, the form the
// REPL's status line took before the ASCII animation.
func (m *replModel) fallbackLine() string {
	frame := fallbackFrames[(m.loaderIdx/fallbackEvery)%len(fallbackFrames)]
	return frame + " " + m.statusText()
}

// statusText is the "<what it's doing> (<elapsed>s)" label shown under the
// animation, and beside the one-character spinner on the hint line.
func (m *replModel) statusText() string {
	status := m.status
	switch {
	case m.streaming && !m.loading():
		// Tokens are arriving, which makes whatever the backend last reported
		// stale — whatever it was doing, it is writing the answer now.
		status = writingStatus
	case status == "":
		status = "working…"
	}
	return fmt.Sprintf("%s (%ds)", status, int(time.Since(m.turnStarted).Seconds()))
}
