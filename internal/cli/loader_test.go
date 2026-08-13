package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/abstraction-dev/cli/internal/render"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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

// TestLoaderViewBand checks the band is exactly the height it claims and that
// its contents are horizontally centred at any width.
func TestLoaderViewBand(t *testing.T) {
	for _, width := range []int{loaderWidth, 46, 80, 200} {
		m := &replModel{turnStarted: time.Now(), loaderIdx: 7, width: width}
		lines := m.loaderRows()

		if len(lines) != loaderBand {
			t.Fatalf("width %d: band is %d rows, want %d", width, len(lines), loaderBand)
		}

		// Everything drawn must sit inside the centred slot, leaving the
		// columns either side of it blank.
		pad := (width - loaderWidth) / 2
		for i, l := range lines {
			if w := lipgloss.Width(l); w > width {
				t.Fatalf("width %d: row %d overflows at %d", width, i, w)
			}
			// Blank rows come back unpadded when the box exactly fits, so pad
			// to the box before slicing the slot out of them.
			r := []rune(ansi.Strip(l))
			for len(r) < width {
				r = append(r, ' ')
			}
			if outside := string(r[:pad]) + string(r[pad+loaderWidth:]); strings.TrimSpace(outside) != "" {
				t.Fatalf("width %d: row %d spills outside the centred slot: %q", width, i, outside)
			}
		}
	}
}

// TestLoaderFits checks the animation is dropped rather than crammed in when
// the window cannot hold it above a usable transcript.
func TestLoaderFits(t *testing.T) {
	for _, tc := range []struct {
		width, vpHeight int
		want            bool
	}{
		{80, 40, true},
		{loaderWidth, loaderBand + loaderMinTranscript, true},
		{loaderWidth - 1, 40, false},                      // too narrow
		{80, loaderBand + loaderMinTranscript - 1, false}, // too short
		{80, loaderBand, false},                           // no conversation left
		{0, 1, false},
	} {
		m := &replModel{width: tc.width}
		m.vp = viewport.New(viewport.WithWidth(tc.width), viewport.WithHeight(tc.vpHeight))
		if got := m.loaderFits(); got != tc.want {
			t.Fatalf("width %d vpHeight %d: fits=%v, want %v", tc.width, tc.vpHeight, got, tc.want)
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

// TestConversationDoesNotMove is the whole point of reserving the block: the
// question stays on exactly the same screen row from the moment a turn starts
// to the moment the answer begins streaming into the rows the animation held.
func TestConversationDoesNotMove(t *testing.T) {
	const question = "and what about sessions?"

	// A conversation long enough to be anchored to the bottom of the viewport,
	// which is the case where a change in height would shunt it around.
	m := newTestModel(t, 80, 28)
	for i := 0; i < 6; i++ {
		m.entries = append(m.entries,
			transcriptEntry{entryUser, "earlier question"},
			transcriptEntry{entryAnswer, "earlier answer"})
	}
	m.entries = append(m.entries, transcriptEntry{entryUser, question})

	// The turn starts and the animation appears below the question.
	m.streaming = true
	m.status = "searching the codebase…"
	m.turnStarted = time.Now()
	settleSpawn(m)

	loadingView := m.vp.View()
	loadingRow := rowOf(t, loadingView, question)
	if !strings.ContainsAny(loadingView, "█▓▒░") {
		t.Fatalf("animation missing while loading:\n%s", loadingView)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "searching the codebase…") {
		t.Fatalf("status missing while loading:\n%s", m.View().Content)
	}

	// The first token lands. The animation goes, the answer takes its place,
	// and the question must not have budged.
	m.answer.WriteString("Sessions are cookie-backed.")
	m.refresh()

	streamingView := m.vp.View()
	if strings.ContainsAny(streamingView, "█▓▒░") {
		t.Fatalf("animation outlived the first token:\n%s", streamingView)
	}
	if got := rowOf(t, streamingView, question); got != loadingRow {
		t.Fatalf("question moved from row %d to row %d when streaming began:\n%s",
			loadingRow, got, streamingView)
	}

	// And it stays put as the answer grows into the space already reserved.
	// Past that the block grows and the view scrolls, which is ordinary
	// streaming rather than something the reservation promises to prevent.
	for i := 0; i < loaderBand; i++ {
		m.answer.WriteString("\n\nanother line")
		m.refresh()
		if len(m.lines)-m.blockStart > loaderBand {
			break
		}
		if got := rowOf(t, m.vp.View(), question); got != loadingRow {
			t.Fatalf("question moved to row %d while the answer grew within the reservation:\n%s",
				got, m.vp.View())
		}
	}
}

// TestReservedBlockHeight checks the block is the same height whether it holds
// the animation or a short answer, which is what keeps the rows above it still.
func TestReservedBlockHeight(t *testing.T) {
	m := newTestModel(t, 80, 28)
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()

	m.refresh()
	if m.blockStart < 0 {
		t.Fatal("no block reserved while loading")
	}
	loadingHeight := len(m.lines)
	if got := len(m.lines) - m.blockStart; got != loaderBand {
		t.Fatalf("block is %d rows while loading, want %d", got, loaderBand)
	}

	m.answer.WriteString("Auth is handled by middleware.")
	m.refresh()
	if got := len(m.lines); got != loadingHeight {
		t.Fatalf("transcript is %d rows once streaming, was %d", got, loadingHeight)
	}

	// Once the answer outgrows the reservation the block grows with it, which
	// is ordinary streaming and should scroll as usual.
	m.answer.WriteString(strings.Repeat("\n\nline", loaderBand))
	m.refresh()
	if got := len(m.lines); got <= loadingHeight {
		t.Fatalf("transcript did not grow past the reservation: %d rows", got)
	}
}

// TestLoaderTooSmallReservesNothing checks a window too short for the animation
// falls back cleanly: no animation, no reservation, spinner on the hint line.
func TestLoaderTooSmallReservesNothing(t *testing.T) {
	m := newTestModel(t, 80, loaderBand+footerHeight) // one row short of fitting
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()
	m.refresh()

	out := m.View().Content
	if strings.ContainsAny(out, "█▓▒░") {
		t.Fatalf("animation drawn in a window too small for it:\n%s", out)
	}
	if m.blockStart >= 0 {
		t.Fatalf("reserved a block in a window too small for it, at %d", m.blockStart)
	}
	if !strings.Contains(ansi.Strip(out), "working… (0s)") {
		t.Fatalf("status missing from the hint line:\n%s", out)
	}
	if !strings.ContainsAny(out, strings.Join(fallbackFrames, "")) {
		t.Fatalf("one-character spinner missing in a window too small for the band:\n%s", out)
	}
}

// TestRefreshLoaderRewritesInPlace checks a tick advances the animation without
// disturbing the transcript around it — the reason ticks are cheap.
func TestRefreshLoaderRewritesInPlace(t *testing.T) {
	m := newTestModel(t, 80, 28)
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()
	m.refresh()

	before := append([]string(nil), m.lines...)
	start := m.blockStart

	m.loaderIdx += 7
	m.refreshLoader()

	if len(m.lines) != len(before) {
		t.Fatalf("tick changed the transcript's height: %d rows, was %d", len(m.lines), len(before))
	}
	for i := 0; i < start; i++ {
		if m.lines[i] != before[i] {
			t.Fatalf("tick rewrote row %d above the block:\n got %q\nwant %q", i, m.lines[i], before[i])
		}
	}
	if strings.Join(m.lines[start:], "\n") == strings.Join(before[start:], "\n") {
		t.Fatal("tick did not advance the animation")
	}
}

// TestDespawn drives the real message path and checks the animation dissolves
// top-to-bottom rather than vanishing, uncovering the answer as it goes, and
// that it is fully gone by the time the dissolve is over.
func TestDespawn(t *testing.T) {
	m := newTestModel(t, 80, 40)
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()
	settleSpawn(m)

	full := artPerRow(m.lines[m.blockStart:])
	if sum(full) == 0 {
		t.Fatalf("no artwork to dissolve:\n%s", strings.Join(m.lines[m.blockStart:], "\n"))
	}

	// The first token starts the dissolve rather than clearing the block.
	var mm tea.Model = m
	mm, _ = mm.Update(deltaMsg("Auth is handled by middleware."))
	if m.despawnStart < 0 {
		t.Fatal("first token did not start the dissolve")
	}
	if !m.despawning() {
		t.Fatal("not dissolving straight after the first token")
	}

	// It eats downwards: the top clears while lower rows still hold artwork,
	// and no row ever gains glyphs back.
	prev := artPerRow(m.lines[m.blockStart:])
	sawPartialRow, sawUnevenFront := false, false
	for step := 0; step < loaderBand*2 && m.despawning(); step++ {
		mm, _ = mm.Update(tickMsg{})
		now := artPerRow(m.lines[m.blockStart:])

		for r := range now {
			if now[r] > prev[r] {
				t.Fatalf("step %d: row %d gained glyphs back (%d -> %d)", step, r, prev[r], now[r])
			}
		}
		// A row part way through breaking up: some glyphs gone, some left.
		for r := range now {
			if now[r] > 0 && now[r] < full[r] {
				sawPartialRow = true
			}
		}
		// Top gone while the bottom is still intact.
		if topArt(now) == 0 && bottomArt(now, full) > 0 {
			sawUnevenFront = true
		}
		prev = now
	}

	if !sawPartialRow {
		t.Fatal("no row ever broke up: the dissolve cut cleanly instead of adding noise")
	}
	if !sawUnevenFront {
		t.Fatal("the dissolve did not run top-to-bottom")
	}
	if m.despawning() {
		t.Fatal("the dissolve never finished")
	}
	if got := artPerRow(m.lines[m.blockStart:]); topArt(got)+bottomArt(got, full) != 0 {
		t.Fatalf("artwork survived the dissolve:\n%s", strings.Join(m.lines[m.blockStart:], "\n"))
	}
	// And the answer it uncovered is what is left in the block.
	if !strings.Contains(ansi.Strip(strings.Join(m.lines[m.blockStart:], "\n")), "Auth is handled by middleware.") {
		t.Fatalf("answer missing after the dissolve:\n%s", strings.Join(m.lines[m.blockStart:], "\n"))
	}
}

// TestSpawn is the dissolve's mirror: the animation assembles from the bottom
// up out of noise rather than appearing whole, and ends up complete.
func TestSpawn(t *testing.T) {
	m := newTestModel(t, 80, 40)
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()
	m.refresh()

	if !m.spawning() {
		t.Fatal("not building at the start of a turn")
	}

	// The animation keeps rotating as it assembles, so each step is compared
	// against the same frame fully formed rather than against a fixed one.
	if now, whole := artPerRow(m.lines[m.blockStart:]), artPerRow(m.loaderRows()); sum(now) >= sum(whole) {
		t.Fatalf("animation was already whole on its first frame: %d of %d glyphs", sum(now), sum(whole))
	}

	sawPartialRow, sawUnevenFront := false, false
	for step := 0; step < loaderBand*2 && m.spawning(); step++ {
		m.loaderIdx++
		m.refreshLoader()
		now, whole := artPerRow(m.lines[m.blockStart:]), artPerRow(m.loaderRows())

		for r := range now {
			if now[r] > 0 && now[r] < whole[r] {
				sawPartialRow = true
			}
		}
		// Bottom filled in while the top is still empty — the opposite of the
		// dissolve, which clears the top first.
		if bottomArt(now, whole) > 0 && topArt(now) == 0 {
			sawUnevenFront = true
		}
	}

	if !sawPartialRow {
		t.Fatal("no row ever formed gradually: the build snapped in instead of gathering out of noise")
	}
	if !sawUnevenFront {
		t.Fatal("the build did not run bottom-to-top")
	}
	if m.spawning() {
		t.Fatal("the build never finished")
	}
	got, whole := artPerRow(m.lines[m.blockStart:]), artPerRow(m.loaderRows())
	if sum(got) != sum(whole) {
		t.Fatalf("animation incomplete after the build: %d of %d glyphs", sum(got), sum(whole))
	}
}

// TestEmergeIsMonotonic checks a glyph, once formed, stays formed as the build
// deepens — the mirror of erode's guarantee, and what stops the noise sparkling.
func TestEmergeIsMonotonic(t *testing.T) {
	rows := strings.Split(loaderFrames[0], "\n")
	row := rows[len(rows)/2]
	if countArt(row) == 0 {
		t.Skip("picked a blank row")
	}

	prev := countArt(emerge(row, 0, 42))
	for depth := 1; depth < buildNoise; depth++ {
		got := countArt(emerge(row, depth, 42))
		if got < prev {
			t.Fatalf("depth %d took glyphs away: %d -> %d", depth, prev, got)
		}
		prev = got
	}
	if prev != countArt(row) {
		t.Fatalf("the deepest build left the row incomplete: %d of %d", prev, countArt(row))
	}
}

// settleSpawn advances the model past the entrance animation, for tests that
// care about the animation running rather than assembling.
func settleSpawn(m *replModel) {
	for m.spawning() {
		m.loaderIdx++
	}
	m.refresh()
}

// TestErodeIsMonotonic checks a glyph, once broken up, stays broken up as the
// dissolve deepens — the property that stops the artwork flickering.
func TestErodeIsMonotonic(t *testing.T) {
	row := loaderFrames[0]
	row = strings.Split(row, "\n")[len(strings.Split(row, "\n"))/2]

	prev := countArt(row)
	if prev == 0 {
		t.Skip("picked a blank row")
	}
	for depth := 0; depth < buildNoise; depth++ {
		got := countArt(erode(row, depth, 42))
		if got > prev {
			t.Fatalf("depth %d put glyphs back: %d -> %d", depth, prev, got)
		}
		prev = got
	}
	if prev == countArt(row) {
		t.Fatal("the deepest erosion removed nothing")
	}
}

func artPerRow(rows []string) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = countArt(r)
	}
	return out
}

func countArt(s string) int {
	n := 0
	for _, r := range ansi.Strip(s) {
		if strings.ContainsRune("█▓▒░", r) {
			n++
		}
	}
	return n
}

// topArt and bottomArt count surviving glyphs in the half of the block each
// side of the artwork's middle, which is enough to tell a top-down dissolve
// from an all-at-once one.
func topArt(counts []int) int { return sum(counts[:loaderHeight/2]) }
func bottomArt(counts, full []int) int {
	return sum(counts[loaderHeight/2 : min(len(counts), loaderHeight)])
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

// rowOf reports which line of a rendered view contains needle.
func rowOf(t *testing.T, view, needle string) int {
	t.Helper()
	for i, l := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(l), needle) {
			return i
		}
	}
	t.Fatalf("%q is not on screen:\n%s", needle, view)
	return -1
}

// TestFallbackLine checks the one-character spinner cycles, and that it is
// slowed back to its original pace despite the faster tick.
func TestFallbackLine(t *testing.T) {
	m := &replModel{turnStarted: time.Now(), status: "thinking…"}
	if got, want := m.fallbackLine(), fallbackFrames[0]+" thinking… (0s)"; got != want {
		t.Fatalf("fallback line: %q, want %q", got, want)
	}

	// A frame is held for fallbackEvery ticks, then the next one shows.
	m.loaderIdx = fallbackEvery - 1
	if got := m.fallbackLine(); !strings.HasPrefix(got, fallbackFrames[0]) {
		t.Fatalf("frame advanced early at tick %d: %q", m.loaderIdx, got)
	}
	m.loaderIdx = fallbackEvery
	if got := m.fallbackLine(); !strings.HasPrefix(got, fallbackFrames[1]) {
		t.Fatalf("frame did not advance at tick %d: %q", m.loaderIdx, got)
	}

	// And it wraps rather than running off the end of the table.
	m.loaderIdx = fallbackEvery * len(fallbackFrames)
	if got := m.fallbackLine(); !strings.HasPrefix(got, fallbackFrames[0]) {
		t.Fatalf("frame did not wrap: %q", got)
	}
}

// TestFallbackWhileStreaming checks the one-character spinner also covers the
// stretch after the band retires, so a turn never looks stalled.
func TestFallbackWhileStreaming(t *testing.T) {
	m := newTestModel(t, 80, 40)
	m.entries = []transcriptEntry{{entryUser, "how does auth work?"}}
	m.streaming = true
	m.turnStarted = time.Now()
	m.answer.WriteString("Auth is handled by middleware.")
	m.refresh()

	out := m.View().Content
	if strings.ContainsAny(out, "█▓▒░") {
		t.Fatalf("band still drawn after the first token:\n%s", out)
	}
	// The spinner sits immediately left of the label, and the label reads as
	// writing rather than whatever the backend last reported.
	plain := ansi.Strip(out)
	if !strings.Contains(plain, writingStatus) {
		t.Fatalf("%q missing while streaming:\n%s", writingStatus, out)
	}
	var found bool
	for _, f := range fallbackFrames {
		if strings.Contains(plain, f+" "+writingStatus) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("spinner is not to the left of %q:\n%s", writingStatus, plain)
	}
}

// TestStatusTextWhileStreaming checks the label switches to writing only once
// tokens are arriving, leaving the backend's own status alone before that.
func TestStatusTextWhileStreaming(t *testing.T) {
	m := &replModel{turnStarted: time.Now(), status: "searching the codebase…"}

	// Before the turn starts, and while it waits on its first token, the
	// backend's status is what the user sees.
	if got, want := m.statusText(), "searching the codebase… (0s)"; got != want {
		t.Fatalf("idle status: %q, want %q", got, want)
	}
	m.streaming = true
	if got, want := m.statusText(), "searching the codebase… (0s)"; got != want {
		t.Fatalf("status while loading: %q, want %q", got, want)
	}

	// Once the answer is coming through it is writing, whatever the backend
	// last said.
	m.answer.WriteString("Auth is handled by middleware.")
	if got, want := m.statusText(), writingStatus+" (0s)"; got != want {
		t.Fatalf("status while streaming: %q, want %q", got, want)
	}
}

func newTestModel(t *testing.T, w, h int) *replModel {
	t.Helper()
	ti := textarea.New()
	ti.ShowLineNumbers = false
	ti.SetHeight(inputHeight)
	m := &replModel{
		env:          &appEnv{workspace: "ws-abcdef123"},
		sub:          make(chan tea.Msg, 8),
		md:           render.NewMDRenderer(true, 0),
		input:        ti,
		blockStart:   -1,
		despawnStart: -1,
	}
	m.applyBackground(true)
	m.resize(w, h)
	return m
}
