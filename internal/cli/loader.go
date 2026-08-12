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

// loaderView renders the current animation frame with the status beneath it,
// centred on both axes in a box of the given size. It reports false when the
// box is too small to hold the animation, leaving the caller on the plain-text
// status instead of a clipped frame.
func (m *replModel) loaderView(width, height int) (string, bool) {
	if len(loaderFrames) == 0 || width < loaderWidth || height < loaderHeight+2 {
		return "", false
	}
	frame := loaderFrames[m.loaderIdx%len(loaderFrames)]
	block := loaderStyle.Render(frame) +
		"\n\n" +
		lipgloss.PlaceHorizontal(loaderWidth, lipgloss.Center, faintStyle.Render(m.statusText()))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block), true
}

// statusText is the "<what it's doing> (<elapsed>s)" label shown under the
// animation, and on the hint line once the answer starts streaming.
func (m *replModel) statusText() string {
	status := m.status
	if status == "" {
		status = "working…"
	}
	return fmt.Sprintf("%s (%ds)", status, int(time.Since(m.turnStarted).Seconds()))
}
