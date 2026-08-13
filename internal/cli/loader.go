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

// buildSpeed is how many rows the front travels per tick as the animation
// assembles and comes apart again, and buildNoise how many ticks a row spends
// part-formed at that front. Together they set each pass's length:
// (loaderBand/buildSpeed) + buildNoise ticks, about a third of a second at
// 30fps. The two passes share them so the entrance and the exit match.
const (
	buildSpeed = 2
	buildNoise = 4
)

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

// spawning reports whether the animation is still assembling itself. The build
// runs from the start of the turn, loaderIdx being reset with it.
func (m *replModel) spawning() bool {
	return m.loading() && m.spawnFront() < loaderBand+buildNoise
}

// spawnFront is how far up from the bottom of the block the build has reached.
// Rows below it are whole, rows above have yet to appear, and the buildNoise
// rows in between are part way through forming.
func (m *replModel) spawnFront() int {
	return m.loaderIdx * buildSpeed
}

// despawning reports whether the animation is still dissolving. The dissolve
// begins the moment the first token lands (see the deltaMsg case in Update) and
// runs on the same tick as the animation itself.
func (m *replModel) despawning() bool {
	return m.despawnStart >= 0 && m.despawnFront() < loaderBand+buildNoise
}

// despawnFront is the row the dissolve has eaten down to. Rows above it are
// gone, rows below untouched, and the buildNoise rows in between are part way
// through breaking up.
func (m *replModel) despawnFront() int {
	return (m.loaderIdx - m.despawnStart) * buildSpeed
}

// blockRows builds the turn's reserved block: the animation assembling itself
// from the bottom up as the turn starts, then running whole, then coming apart
// from the top down over the answer once the first token lands, and from then
// on the answer padded out to the block's fixed height.
func (m *replModel) blockRows(answer []string) []string {
	switch {
	case m.spawning():
		return m.spawnRows()
	case m.loading():
		return m.loaderRows()
	case m.despawning():
		return m.despawnRows(answer)
	}
	rows := append([]string(nil), answer...)
	for len(rows) < loaderBand {
		rows = append(rows, "")
	}
	return rows
}

// loaderRows renders the current animation frame with the status beneath it,
// centred horizontally, as exactly loaderBand rows. That fixed height is what
// keeps the conversation still: the answer later streams into the same rows.
func (m *replModel) loaderRows() []string {
	plain := m.loaderPlain(m.loaderIdx)
	rows := make([]string, loaderBand)
	for i := range rows {
		rows[i] = m.renderBlockRow(i, plain[i])
	}
	return rows
}

// spawnRows renders the animation part way through assembling, building up
// from the bottom of the block so the status line lands first and the artwork
// gathers above it.
func (m *replModel) spawnRows() []string {
	plain := m.loaderPlain(m.loaderIdx)
	front := m.spawnFront()

	rows := make([]string, loaderBand)
	for i := range rows {
		// Distance from the bottom, the build running the other way to the
		// dissolve.
		switch depth := front - (loaderBand - 1 - i); {
		case depth >= buildNoise:
			rows[i] = m.renderBlockRow(i, plain[i])
		case depth < 0:
			rows[i] = "" // not arrived yet
		default:
			rows[i] = m.renderBlockRow(i, emerge(plain[i], depth, m.buildSeed()+i))
		}
	}
	return rows
}

// despawnRows renders the animation part way through dissolving, with the
// answer showing through the rows it has already cleared.
func (m *replModel) despawnRows(answer []string) []string {
	// Frozen on the frame the dissolve began: the artwork is coming apart, not
	// still turning, and a row cannot regain glyphs from a later frame.
	plain := m.loaderPlain(m.despawnStart)
	front := m.despawnFront()

	rows := make([]string, loaderBand)
	for i := range rows {
		switch depth := front - i; {
		case depth >= buildNoise:
			// Nothing of the artwork left here; the answer has the row.
			rows[i] = rowAt(answer, i)
		case depth < 0:
			rows[i] = m.renderBlockRow(i, plain[i])
		default:
			rows[i] = m.renderBlockRow(i, erode(plain[i], depth, m.despawnStart+i))
		}
	}
	// An answer that already overflows the block keeps streaming below it.
	if len(answer) > loaderBand {
		rows = append(rows, answer[loaderBand:]...)
	}
	return rows
}

// loaderPlain returns the block's rows before styling: the given frame of the
// rotation, a blank row, and the status line.
func (m *replModel) loaderPlain(frameIdx int) []string {
	frame := loaderFrames[frameIdx%len(loaderFrames)]
	rows := make([]string, 0, loaderBand)
	rows = append(rows, strings.Split(frame, "\n")...)
	return append(rows, "", m.statusText())
}

// renderBlockRow styles one of the block's rows and centres it in the window.
// Frame rows are all loaderWidth wide, so they centre as a single column; the
// status is narrower and centres on its own.
func (m *replModel) renderBlockRow(i int, row string) string {
	style := loaderStyle
	if i >= loaderHeight {
		style = faintStyle
	}
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, style.Render(row))
}

// erode blanks out a share of a row's glyphs, more of them the deeper the
// dissolve has got into it, so the artwork breaks up instead of being cut off
// along a clean line.
func erode(row string, depth, seed int) string {
	threshold := buildThreshold(depth)
	out := []rune(row)
	for i, r := range out {
		if r != ' ' && noiseAt(seed, i) < threshold {
			out[i] = ' '
		}
	}
	return string(out)
}

// emerge is erode's mirror: it holds back a share of a row's glyphs, fewer of
// them the deeper the build has got, so the artwork gathers out of noise
// instead of snapping into place whole.
func emerge(row string, depth, seed int) string {
	threshold := buildThreshold(depth)
	out := []rune(row)
	for i, r := range out {
		if r != ' ' && noiseAt(seed, i) >= threshold {
			out[i] = ' '
		}
	}
	return string(out)
}

// buildThreshold is the share of glyphs, out of 256, that a front at the given
// depth has got to. Both passes compare the same per-cell noise against it, so
// each is monotonic: a glyph that has gone stays gone, one that has arrived
// stays put. The last depth reaches 256 so a row is complete by the time the
// front leaves it, and there is no jump as it passes.
func buildThreshold(depth int) int {
	return (depth + 1) * 256 / buildNoise
}

// buildSeed varies the noise from turn to turn, so the animation does not
// assemble in exactly the same pattern every time, while staying fixed for the
// length of any one build.
func (m *replModel) buildSeed() int {
	return len(m.entries)
}

// noiseAt is a cheap deterministic hash in [0,256). A cell's fate depends only
// on where it is and which dissolve it belongs to, never on when the frame was
// drawn — so a glyph that has broken up stays broken up rather than flickering
// back on the next tick.
func noiseAt(seed, i int) int {
	h := uint32(seed)*2654435761 + uint32(i)*40503
	h ^= h >> 13
	h *= 1274126177
	h ^= h >> 16
	return int(h & 0xff)
}

func rowAt(rows []string, i int) string {
	if i < len(rows) {
		return rows[i]
	}
	return ""
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
