package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abstraction-dev/cli/internal/apiclient"
	"github.com/abstraction-dev/cli/internal/render"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// inputHeight is the number of visible rows in the input box. It wraps long
// queries within its width and scrolls internally rather than growing, so the
// rest of the layout never has to reflow around it.
const inputHeight = 3

// footerHeight is the number of lines reserved below the transcript viewport:
// a hint/status line, a rule, the input box, another rule, and the context bar.
const footerHeight = inputHeight + 4

// spinnerInterval is how often the in-transcript status line's spinner frame
// and elapsed timer advance while a turn is streaming.
const spinnerInterval = 100 * time.Millisecond

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// inputPrompt marks the first line of the input box; wrapped continuation
// lines get no prompt (see the SetPromptFunc call in runREPL).
const inputPrompt = "❯ "

var (
	userStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	faintStyle = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	labelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	// selectedStyle marks mouse-selected transcript text. Reverse video is what
	// terminals use for their own selections, and it inverts whatever colour the
	// text already had rather than needing a palette that suits every case.
	selectedStyle = lipgloss.NewStyle().Reverse(true)
)

// escLeakPattern matches terminal capability-query RESPONSES (OSC 10/11 color
// reports, cursor-position and device-attribute reports). Some terminals answer
// these queries right as a program acquires the terminal, and a response that
// isn't parsed as an event lands in the text input instead. We strip any such
// fragment defensively so it never corrupts a query or command.
var escLeakPattern = regexp.MustCompile(`\]1[01];rgb:[0-9a-fA-F/]+|\[\??[0-9;]*[cuR]`)

// runREPL launches the interactive session as a full-screen TUI: an alt-screen
// transcript (ANSI markdown) above a context bar and prompt. Esc or Ctrl+C
// cancels the in-flight question (Ctrl+C also clears the input when idle);
// Ctrl+D or /exit quits. Up/Down
// navigate an in-memory history for this run.
func runREPL(env *appEnv, initialPR string) int {
	ti := textarea.New()
	ti.Prompt = inputPrompt
	ti.SetPromptFunc(lipgloss.Width(inputPrompt), func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return inputPrompt
		}
		return ""
	})
	ti.Placeholder = "Ask Astrid…  (/help, /exit)"
	ti.ShowLineNumbers = false
	ti.CharLimit = 8192
	ti.SetHeight(inputHeight)
	ti.Focus()

	// sessionID is left empty: the first question starts a conversation and the
	// backend names it, which the REPL then holds onto (see conversationMsg).
	//
	// The dark palette is a placeholder. Querying the terminal from here would
	// leak the response into the input, so the program asks for the background
	// itself and restyles when the answer arrives (see tea.BackgroundColorMsg).
	m := &replModel{
		env:      env,
		sub:      make(chan tea.Msg, 256),
		md:       render.NewMDRenderer(true, 0),
		input:    ti,
		activePR: initialPR,
	}
	m.applyBackground(true)
	m.entries = []transcriptEntry{{entrySystem, "Chatting with Astrid. Type /help for commands, /exit to quit."}}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(env.render.Err, "abstr: "+err.Error())
		return exitRuntime
	}
	return exitOK
}

// applyBackground fits the markdown renderer and the input box's styling to the
// terminal background. Light/dark is a single explicit choice made here, not
// resolved per style.
func (m *replModel) applyBackground(dark bool) {
	m.md.SetDark(dark)

	st := textarea.DefaultStyles(dark)
	st.Focused.Prompt = userStyle
	st.Blurred.Prompt = userStyle
	// No active-line background highlight — keep the plain look the
	// single-line input had.
	st.Focused.CursorLine = lipgloss.NewStyle()
	m.input.SetStyles(st)
}

// Messages delivered over m.sub by the ask goroutine.
type (
	deltaMsg  string
	statusMsg string
	doneMsg   struct{ err error }
	// conversationMsg carries the conversation the backend ran this turn in, which
	// every later question is sent under. On the first question of a conversation it
	// is the only place its id appears.
	conversationMsg string
)

// tickMsg advances the in-transcript spinner/elapsed-timer while a turn streams.
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// One-shot command results (not part of the m.sub stream).
type (
	nameResolvedMsg struct{ name string }
	pickerLoadedMsg struct {
		items []apiclient.Workspace
		err   error
	}
	switchResultMsg struct {
		slug, name string
		err        error
	}
	prPickerLoadedMsg struct {
		items []apiclient.PRReview
		err   error
	}
	prSetResultMsg struct {
		pr    string // value stored as the scope (the PR's URL)
		label string // friendly label for the transcript
		err   error
	}
	chatPickerLoadedMsg struct {
		items []apiclient.Chat
		err   error
	}
	chatLoadedMsg struct {
		chat apiclient.ChatWithMessages
		err  error
	}
)

type replMode int

const (
	modeNormal replMode = iota
	modePicking
	modePickingPR
	modePickingChat
)

type entryKind int

const (
	entryUser entryKind = iota
	entryAnswer
	entrySystem
	entryError
)

type transcriptEntry struct {
	kind entryKind
	text string
}

type replModel struct {
	env *appEnv
	sub chan tea.Msg
	md  *render.MDRenderer

	vp    viewport.Model
	input textarea.Model
	ready bool
	width int

	entries []transcriptEntry
	// lines is the transcript as rendered, kept line by line so a mouse
	// selection can be painted over it and copied out of it (see selection.go).
	lines  []string
	sel    selection
	copied bool // the last selection reached the clipboard; shown in the hint

	wsName string // resolved friendly name of the current workspace

	history []string
	histIdx int
	draft   string

	sessionID string
	activePR  string

	streaming   bool
	answer      strings.Builder
	status      string
	cancel      context.CancelFunc
	spinnerIdx  int
	turnStarted time.Time

	mode        replMode
	pickerItems []apiclient.Workspace
	pickerIdx   int
	prItems     []apiclient.PRReview
	prIdx       int
	chatItems   []apiclient.Chat
	chatIdx     int
}

func waitForMsg(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-sub }
}

func (m *replModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.RequestBackgroundColor, waitForMsg(m.sub), m.resolveCurrentNameCmd())
}

func (m *replModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.BackgroundColorMsg:
		// The answer to Init's query: restyle for the real background and redraw
		// whatever was already rendered under the placeholder palette.
		m.applyBackground(msg.IsDark())
		m.refreshCurrent()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseWheelMsg:
		// The wheel scrolls the transcript. A selection is held in transcript
		// coordinates, so scrolling moves it with the text rather than losing it.
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd

	case tea.MouseClickMsg:
		return m.startSelection(tea.Mouse(msg))
	case tea.MouseMotionMsg:
		return m.extendSelection(tea.Mouse(msg))
	case tea.MouseReleaseMsg:
		return m.finishSelection()

	case tickMsg:
		if !m.streaming {
			return m, nil
		}
		m.spinnerIdx++
		m.refresh()
		return m, tickCmd()

	case conversationMsg:
		m.sessionID = string(msg)
		return m, waitForMsg(m.sub)
	case statusMsg:
		m.status = string(msg)
		m.refresh()
		return m, waitForMsg(m.sub)
	case deltaMsg:
		m.answer.WriteString(string(msg))
		m.refresh()
		return m, waitForMsg(m.sub)
	case doneMsg:
		m.finishTurn(msg.err)
		return m, waitForMsg(m.sub)

	case nameResolvedMsg:
		if msg.name != "" {
			m.wsName = msg.name
		}
		return m, nil
	case pickerLoadedMsg:
		m.openPicker(msg)
		return m, nil
	case switchResultMsg:
		if msg.err != nil {
			m.addEntry(entryError, msg.err.Error())
		} else {
			m.applySwitch(msg.slug, msg.name)
		}
		return m, nil
	case prPickerLoadedMsg:
		m.openPRPicker(msg)
		return m, nil
	case prSetResultMsg:
		if msg.err != nil {
			m.addEntry(entryError, msg.err.Error())
		} else {
			m.applyPRScope(msg.pr, msg.label)
		}
		return m, nil
	case chatPickerLoadedMsg:
		m.openChatPicker(msg)
		return m, nil
	case chatLoadedMsg:
		if msg.err != nil {
			m.addEntry(entryError, msg.err.Error())
		} else {
			m.resumeConversation(msg.chat)
		}
		// Typing is allowed again either way: on a failure the conversation being
		// held is still the one questions go to.
		m.input.Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *replModel) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modePicking:
		return m.handlePickerKey(key)
	case modePickingPR:
		return m.handlePRPickerKey(key)
	case modePickingChat:
		return m.handleChatPickerKey(key)
	}

	switch key.String() {
	case "ctrl+r":
		// Reverse-search's shell mnemonic: reach back for an earlier conversation.
		if m.streaming {
			return m, nil
		}
		return m, m.loadChatPickerCmd()

	case "ctrl+c":
		if m.streaming {
			m.status = "cancelling…"
			if m.cancel != nil {
				m.cancel()
			}
			m.refresh()
			return m, nil
		}
		m.input.Reset()
		m.histIdx = len(m.history)
		return m, nil

	case "ctrl+d":
		if !m.streaming && m.input.Value() == "" {
			return m, tea.Quit
		}
		return m, nil

	case "esc":
		// Esc drops a selection as well as cancelling: with no terminal-drawn
		// selection to click away from, this is the way to dismiss one.
		m.clearSelection()
		if m.streaming {
			m.status = "cancelling…"
			if m.cancel != nil {
				m.cancel()
			}
			m.refresh()
		}
		return m, nil

	case "enter":
		if m.streaming {
			return m, nil
		}
		return m.submit()

	case "up":
		if !m.streaming {
			m.historyPrev()
		}
		return m, nil
	case "down":
		if !m.streaming {
			m.historyNext()
		}
		return m, nil

	case "pgup", "pgdown":
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(key)
		return m, cmd
	}

	// While the agent is responding the input is disabled.
	if m.streaming {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	m.sanitizeInput()
	return m, cmd
}

// sanitizeInput strips any leaked terminal-response fragment from the input.
func (m *replModel) sanitizeInput() {
	v := m.input.Value()
	if cleaned := escLeakPattern.ReplaceAllString(v, ""); cleaned != v {
		m.input.SetValue(cleaned)
		m.input.CursorEnd()
	}
}

func (m *replModel) handlePickerKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up":
		if m.pickerIdx > 0 {
			m.pickerIdx--
			m.refreshPicker()
		}
	case "down":
		if m.pickerIdx < len(m.pickerItems)-1 {
			m.pickerIdx++
			m.refreshPicker()
		}
	case "enter":
		w := m.pickerItems[m.pickerIdx]
		m.mode = modeNormal
		m.input.Focus()
		m.applySwitch(w.Slug, w.Name)
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Focus()
		m.addEntry(entrySystem, "workspace selection cancelled")
	}
	return m, nil
}

func (m *replModel) handlePRPickerKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up":
		if m.prIdx > 0 {
			m.prIdx--
			m.refreshPRPicker()
		}
	case "down":
		if m.prIdx < len(m.prItems)-1 {
			m.prIdx++
			m.refreshPRPicker()
		}
	case "enter":
		pr := m.prItems[m.prIdx]
		m.mode = modeNormal
		m.input.Focus()
		if !pr.Ready() {
			m.addEntry(entrySystem, fmt.Sprintf("PR #%d is not ready yet (%s) — pick a completed review", pr.PRNumber, prStatusLabel(pr.Status)))
			return m, nil
		}
		m.applyPRScope(pr.PRURL, prLabel(pr))
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Focus()
		m.addEntry(entrySystem, "PR selection cancelled")
	}
	return m, nil
}

func (m *replModel) handleChatPickerKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up":
		if m.chatIdx > 0 {
			m.chatIdx--
			m.refreshChatPicker()
		}
	case "down":
		if m.chatIdx < len(m.chatItems)-1 {
			m.chatIdx++
			m.refreshChatPicker()
		}
	case "enter":
		chat := m.chatItems[m.chatIdx]
		m.mode = modeNormal
		// The history is a second request, and the transcript is replayed when it
		// arrives (see resumeConversation), so the input stays blurred until then —
		// a question typed into the gap would go to the conversation being left.
		m.input.Blur()
		m.addEntry(entrySystem, "loading "+chatLabel(chat)+"…")
		return m, m.loadChatCmd(chat)
	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Focus()
		m.addEntry(entrySystem, "conversation selection cancelled")
	}
	return m, nil
}

func (m *replModel) submit() (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(escLeakPattern.ReplaceAllString(m.input.Value(), ""))
	if q == "" {
		return m, nil
	}
	m.history = append(m.history, q)
	m.histIdx = len(m.history)
	m.draft = ""
	m.input.Reset()
	// A fresh submission jumps to the latest, even if the user had scrolled up.
	m.vp.GotoBottom()

	if strings.HasPrefix(q, "/") {
		return m.runCommand(q)
	}
	m.addEntry(entryUser, q)
	return m, m.startTurn(q)
}

func (m *replModel) runCommand(cmd string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(cmd)
	switch fields[0] {
	case "/exit", "/quit":
		return m, tea.Quit
	case "/help":
		m.addEntry(entrySystem, replHelp)
	case "/new", "/reset":
		m.newConversation()
		m.addEntry(entrySystem, "started a new conversation")
	case "/chats", "/resume":
		return m, m.loadChatPickerCmd()
	case "/pr":
		switch {
		case len(fields) < 2:
			return m, m.loadPRPickerCmd() // open the PR list
		case fields[1] == "clear":
			m.activePR = ""
			m.newConversation()
			m.addEntry(entrySystem, "PR scope cleared")
		default:
			return m, m.setPRCmd(fields[1]) // validate readiness + set
		}
	case "/workspace", "/ws":
		if len(fields) >= 2 {
			return m, m.switchWorkspaceCmd(fields[1]) // validate + switch
		}
		return m, m.loadPickerCmd() // open picker
	default:
		m.addEntry(entrySystem, "unknown command: "+fields[0]+" (try /help)")
	}
	return m, nil
}

// startTurn kicks off the streaming ask; input is disabled until it completes.
func (m *replModel) startTurn(query string) tea.Cmd {
	// Cancel any still-running turn so its output/status/done frames can't
	// interleave with the new one over the shared m.sub channel.
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.streaming = true
	m.answer.Reset()
	m.status = "thinking…"
	m.spinnerIdx = 0
	m.turnStarted = time.Now()
	m.input.Blur()
	m.refresh()

	req := apiclient.AskRequest{
		Workspace: m.env.workspace,
		Question:  query,
		PR:        m.activePR,
		SessionID: m.sessionID,
	}
	sub := m.sub
	client := m.env.client
	go func() {
		err := client.AskStream(ctx, req, apiclient.StreamHandlers{
			OnOutput:       func(t string) { sub <- deltaMsg(t) },
			OnStatus:       func(s string) { sub <- statusMsg(s) },
			OnConversation: func(slug string) { sub <- conversationMsg(slug) },
		})
		sub <- doneMsg{err: err}
	}()
	return tickCmd()
}

func (m *replModel) finishTurn(err error) {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.streaming = false
	m.status = ""

	if ans := m.answer.String(); strings.TrimSpace(ans) != "" {
		m.entries = append(m.entries, transcriptEntry{entryAnswer, ans})
	}
	switch {
	case err != nil && isCanceled(err):
		m.entries = append(m.entries, transcriptEntry{entrySystem, "(cancelled)"})
	case err != nil:
		m.entries = append(m.entries, transcriptEntry{entryError, err.Error()})
	}
	m.answer.Reset()
	m.input.Focus()
	m.refresh()
}

// newConversation lets go of the current conversation and flushes the transcript.
// Holding no conversation is what makes the next question start one — the backend
// names it and the REPL adopts that name, the same way the first question of a run
// does.
func (m *replModel) newConversation() {
	m.sessionID = ""
	m.entries = nil
	// The lines the selection pointed at are gone with the transcript.
	m.sel = selection{}
	m.copied = false
	m.refresh()
}

// resumeConversation makes a stored conversation the active one and replays it into
// the transcript.
//
// The session id becomes the conversation's slug: the backend continues whatever
// conversation the id names, so every question from here on lands in this one —
// which is the same conversation the app shows. A PR conversation also restores its
// scope, so the status bar says what the answers are grounded in and starting a new
// conversation keeps that grounding.
func (m *replModel) resumeConversation(loaded apiclient.ChatWithMessages) {
	m.sessionID = loaded.Chat.Slug
	m.activePR = loaded.Chat.DiffReportID
	m.entries = nil
	// A selection points at line numbers in the transcript being replaced.
	m.sel = selection{}
	m.copied = false

	unreadable := 0
	for _, msg := range loaded.Messages {
		turns, err := msg.Turns()
		if err != nil {
			unreadable++
			continue
		}
		for _, turn := range turns {
			switch turn.Role {
			case apiclient.TurnUser:
				m.entries = append(m.entries, transcriptEntry{entryUser, turn.Text})
				// Past questions join this run's input history, so ↑ recalls them the
				// way it recalls the ones typed here.
				m.history = append(m.history, turn.Text)
			case apiclient.TurnAssistant:
				m.entries = append(m.entries, transcriptEntry{entryAnswer, turn.Text})
			}
		}
	}
	m.histIdx = len(m.history)

	m.addEntry(entrySystem, "resumed conversation: "+chatLabel(loaded.Chat))
	if unreadable > 0 {
		m.addEntry(entrySystem, fmt.Sprintf("%d earlier exchange(s) could not be replayed — the agent still has them", unreadable))
	}
	if m.ready {
		// Land on the latest turn: a long conversation is read from the bottom, the
		// same place a live answer arrives.
		m.vp.GotoBottom()
	}
}

func (m *replModel) applySwitch(slug, name string) {
	// Persist first; only advance in-memory state if the save succeeds, so a
	// failed write never leaves the session ahead of what's on disk.
	prev := m.env.cfg.Workspace
	m.env.cfg.Workspace = slug
	if err := m.env.cfg.Save(); err != nil {
		m.env.cfg.Workspace = prev
		m.addEntry(entryError, "could not save config: "+err.Error())
		return
	}
	m.env.workspace = slug
	m.wsName = name
	m.activePR = ""
	m.newConversation()
	m.addEntry(entrySystem, "switched to workspace "+workspaceLabel(name, slug))
}

func (m *replModel) openPicker(msg pickerLoadedMsg) {
	if msg.err != nil {
		m.addEntry(entryError, msg.err.Error())
		return
	}
	if len(msg.items) == 0 {
		m.addEntry(entrySystem, "no workspaces found for this account")
		return
	}
	m.mode = modePicking
	m.pickerItems = msg.items
	m.pickerIdx = 0
	for i, w := range msg.items {
		if w.Slug == m.env.workspace {
			m.pickerIdx = i
			break
		}
	}
	m.input.Blur()
	m.refreshPicker()
}

func (m *replModel) openPRPicker(msg prPickerLoadedMsg) {
	if msg.err != nil {
		m.addEntry(entryError, msg.err.Error())
		return
	}
	if len(msg.items) == 0 {
		m.addEntry(entrySystem, "no pull requests found for this workspace")
		return
	}
	m.mode = modePickingPR
	m.prItems = msg.items
	m.prIdx = 0
	m.input.Blur()
	m.refreshPRPicker()
}

func (m *replModel) openChatPicker(msg chatPickerLoadedMsg) {
	if msg.err != nil {
		m.addEntry(entryError, msg.err.Error())
		return
	}
	if len(msg.items) == 0 {
		m.addEntry(entrySystem, "no earlier conversations in this workspace")
		return
	}
	m.mode = modePickingChat
	m.chatItems = msg.items
	m.chatIdx = 0
	// Start on the conversation already being held, if this is one; a conversation
	// started here has an id no stored conversation carries, so nothing preselects.
	for i, chat := range msg.items {
		if chat.Slug == m.sessionID {
			m.chatIdx = i
			break
		}
	}
	m.input.Blur()
	m.refreshChatPicker()
}

// applyPRScope stores the chosen PR as the active scope and starts a fresh
// conversation, so the new scope isn't mixed with prior turns.
func (m *replModel) applyPRScope(pr, label string) {
	m.activePR = pr
	m.newConversation()
	m.addEntry(entrySystem, "PR scope set: "+label)
}

// --- async command Cmds ---

func (m *replModel) resolveCurrentNameCmd() tea.Cmd {
	client, slug := m.env.client, m.env.workspace
	return func() tea.Msg {
		items, err := client.Workspaces(context.Background())
		if err != nil {
			return nameResolvedMsg{}
		}
		for _, w := range items {
			if w.Slug == slug {
				return nameResolvedMsg{name: w.Name}
			}
		}
		return nameResolvedMsg{}
	}
}

func (m *replModel) loadPickerCmd() tea.Cmd {
	client := m.env.client
	return func() tea.Msg {
		items, err := client.Workspaces(context.Background())
		return pickerLoadedMsg{items: items, err: err}
	}
}

func (m *replModel) switchWorkspaceCmd(target string) tea.Cmd {
	client := m.env.client
	return func() tea.Msg {
		items, err := client.Workspaces(context.Background())
		if err != nil {
			return switchResultMsg{err: err}
		}
		for _, w := range items {
			if w.Slug == target || strings.EqualFold(w.Name, target) {
				return switchResultMsg{slug: w.Slug, name: w.Name}
			}
		}
		return switchResultMsg{err: fmt.Errorf("no accessible workspace matching %q", target)}
	}
}

func (m *replModel) loadPRPickerCmd() tea.Cmd {
	client, ws := m.env.client, m.env.workspace
	return func() tea.Msg {
		items, err := client.PRReviews(context.Background(), ws)
		return prPickerLoadedMsg{items: items, err: err}
	}
}

func (m *replModel) loadChatPickerCmd() tea.Cmd {
	client, ws := m.env.client, m.env.workspace
	return func() tea.Msg {
		items, err := client.ListChats(context.Background(), ws)
		return chatPickerLoadedMsg{items: items, err: err}
	}
}

// loadChatCmd fetches the chosen conversation's stored history, which the transcript
// is rebuilt from.
func (m *replModel) loadChatCmd(chat apiclient.Chat) tea.Cmd {
	client := m.env.client
	return func() tea.Msg {
		loaded, err := client.GetChat(context.Background(), chat.Slug)
		return chatLoadedMsg{chat: loaded, err: err}
	}
}

// setPRCmd resolves a pasted GitHub PR URL against the workspace's reviews and
// only scopes to it when the review is ready (completed). A URL with no
// matching or not-yet-ready review is reported, not set. Bare PR numbers are
// intentionally not accepted here — use the picker (`/pr` with no argument).
func (m *replModel) setPRCmd(url string) tea.Cmd {
	client, ws := m.env.client, m.env.workspace
	return func() tea.Msg {
		if !isPRURL(url) {
			return prSetResultMsg{err: fmt.Errorf("%q is not a pull request URL — paste a GitHub PR URL, or run /pr to pick from the list", url)}
		}
		reviews, err := client.PRReviews(context.Background(), ws)
		if err != nil {
			return prSetResultMsg{err: err}
		}
		pr := matchPRReview(reviews, url)
		if pr == nil {
			return prSetResultMsg{err: fmt.Errorf("no pull request matching %q in this workspace", url)}
		}
		if !pr.Ready() {
			return prSetResultMsg{err: fmt.Errorf("PR #%d is not ready yet (%s) — its review must be completed before scoping to it", pr.PRNumber, prStatusLabel(pr.Status))}
		}
		return prSetResultMsg{pr: pr.PRURL, label: prLabel(*pr)}
	}
}

// --- PR URL matching ---

var prURLNumberRe = regexp.MustCompile(`/pull/(\d+)`)

// isPRURL reports whether s looks like a GitHub pull request URL (contains a
// /pull/<n> segment). Bare PR numbers are deliberately rejected.
func isPRURL(s string) bool {
	return prURLNumberRe.FindStringIndex(s) != nil
}

// matchPRReview finds the review a pasted PR URL points at — first by
// normalised URL equality (so http/https and trailing-slash variants resolve),
// then by the PR number in its /pull/<n> path as a fallback.
func matchPRReview(reviews []apiclient.PRReview, url string) *apiclient.PRReview {
	norm := normalizePRURL(url)
	for i := range reviews {
		if normalizePRURL(reviews[i].PRURL) == norm {
			return &reviews[i]
		}
	}
	if m := prURLNumberRe.FindStringSubmatch(url); m != nil {
		n, _ := strconv.Atoi(m[1])
		return prByNumber(reviews, n)
	}
	return nil
}

func prByNumber(reviews []apiclient.PRReview, n int) *apiclient.PRReview {
	for i := range reviews {
		if reviews[i].PRNumber == n {
			return &reviews[i]
		}
	}
	return nil
}

// normalizePRURL lowercases and strips the scheme and trailing slash so URLs
// that differ only cosmetically compare equal.
func normalizePRURL(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimSuffix(u, "/")
}

// prLabel is the transcript label for a selected PR, e.g. "#123 · Fix the bug".
func prLabel(pr apiclient.PRReview) string {
	if pr.PRTitle == "" {
		return fmt.Sprintf("#%d", pr.PRNumber)
	}
	return fmt.Sprintf("#%d · %s", pr.PRNumber, pr.PRTitle)
}

// prStatusLabel renders a review's raw status for humans, e.g. "in progress".
func prStatusLabel(status string) string {
	if status == "" {
		return "pending"
	}
	return strings.ToLower(strings.ReplaceAll(status, "_", " "))
}

// --- history ---

func (m *replModel) historyPrev() {
	if len(m.history) == 0 || m.histIdx == 0 {
		return
	}
	if m.histIdx == len(m.history) {
		m.draft = m.input.Value()
	}
	m.histIdx--
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

func (m *replModel) historyNext() {
	if m.histIdx >= len(m.history) {
		return
	}
	m.histIdx++
	if m.histIdx == len(m.history) {
		m.input.SetValue(m.draft)
	} else {
		m.input.SetValue(m.history[m.histIdx])
	}
	m.input.CursorEnd()
}

// --- rendering ---

func (m *replModel) resize(w, h int) {
	m.width = w
	vpHeight := h - footerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	if !m.ready {
		m.vp = viewport.New(viewport.WithWidth(w), viewport.WithHeight(vpHeight))
		m.ready = true
	} else {
		m.vp.SetWidth(w)
		m.vp.SetHeight(vpHeight)
	}
	m.input.SetWidth(w - 4)
	m.md.Resize(w - 2)
	m.refreshCurrent()
}

// refreshCurrent redraws whichever view is on screen.
func (m *replModel) refreshCurrent() {
	switch m.mode {
	case modePicking:
		m.refreshPicker()
	case modePickingPR:
		m.refreshPRPicker()
	case modePickingChat:
		m.refreshChatPicker()
	default:
		m.refresh()
	}
}

func (m *replModel) addEntry(kind entryKind, text string) {
	m.entries = append(m.entries, transcriptEntry{kind, text})
	m.refresh()
}

func (m *replModel) refresh() {
	if !m.ready {
		return
	}
	// Follow new content only when the user is already at the bottom; if they
	// scrolled up to read earlier output, don't yank them back down.
	stick := m.vp.AtBottom()

	var b strings.Builder
	for i, e := range m.entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.renderEntry(e))
	}
	if m.streaming {
		if live := strings.TrimSpace(m.answer.String()); live != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(strings.TrimRight(m.md.Render(m.answer.String()), "\n"))
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.spinnerLine())
	}
	m.lines = strings.Split(b.String(), "\n")
	m.paint()
	if stick {
		m.vp.GotoBottom()
	}
}

// paint pushes the transcript into the viewport with any selection highlighted.
// It is the only place the viewport's content is set, so a redraw can be asked
// for by a selection change alone, without re-rendering every entry.
func (m *replModel) paint() {
	m.vp.SetContent(strings.Join(paintSelection(m.lines, m.sel), "\n"))
}

// pointAt maps a screen position onto the transcript, reporting false when it
// falls outside the transcript's part of the frame. The viewport starts at the
// top of the frame, so the row is the offset into the visible window; adding
// the scroll position turns it into a position in the transcript as a whole.
func (m *replModel) pointAt(x, y int) (selPoint, bool) {
	if m.mode != modeNormal || y < 0 || y >= m.vp.Height() {
		return selPoint{}, false
	}
	return selPoint{line: m.vp.YOffset() + y, col: x}, true
}

func (m *replModel) startSelection(e tea.Mouse) (tea.Model, tea.Cmd) {
	if e.Button != tea.MouseLeft {
		return m, nil
	}
	p, ok := m.pointAt(e.X, e.Y)
	if !ok {
		// Pressing outside the transcript dismisses whatever was selected, the
		// way clicking away from a selection does anywhere else.
		m.clearSelection()
		return m, nil
	}
	// Not active until the pointer actually moves: a bare click selects nothing.
	m.sel = selection{anchor: p, focus: p, dragging: true}
	m.copied = false
	m.paint()
	return m, nil
}

func (m *replModel) extendSelection(e tea.Mouse) (tea.Model, tea.Cmd) {
	if !m.sel.dragging {
		return m, nil
	}
	p, ok := m.pointAt(e.X, e.Y)
	if !ok {
		return m, nil
	}
	m.sel.focus = p
	m.sel.active = p != m.sel.anchor
	m.paint()
	return m, nil
}

func (m *replModel) finishSelection() (tea.Model, tea.Cmd) {
	if !m.sel.dragging {
		return m, nil
	}
	m.sel.dragging = false

	text := selectedText(m.lines, m.sel)
	if text == "" {
		m.clearSelection()
		return m, nil
	}
	// The terminal never saw the drag, so it has nothing to put on the
	// clipboard; OSC 52 is how the selection actually gets there.
	m.copied = true
	return m, tea.SetClipboard(text)
}

// clearSelection drops the selection and repaints if anything was showing.
func (m *replModel) clearSelection() {
	if m.sel == (selection{}) && !m.copied {
		return
	}
	m.sel = selection{}
	m.copied = false
	m.paint()
}

// spinnerLine renders the animated "<frame> <status> (<elapsed>s)" line shown
// in the transcript, right under the latest message, while a turn streams.
func (m *replModel) spinnerLine() string {
	frame := spinnerFrames[m.spinnerIdx%len(spinnerFrames)]
	status := m.status
	if status == "" {
		status = "working…"
	}
	elapsed := int(time.Since(m.turnStarted).Seconds())
	return faintStyle.Render(fmt.Sprintf("%s %s (%ds)    esc cancels", frame, status, elapsed))
}

func (m *replModel) renderEntry(e transcriptEntry) string {
	switch e.kind {
	case entryUser:
		return userStyle.Render("❯ " + e.text)
	case entryAnswer:
		return strings.TrimRight(m.md.Render(e.text), "\n")
	case entryError:
		return errStyle.Render("error: " + e.text)
	default:
		return faintStyle.Render(e.text)
	}
}

func (m *replModel) refreshPicker() {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Select a workspace") + "\n\n")
	for i, w := range m.pickerItems {
		tag := ""
		if w.IsDefault {
			tag = faintStyle.Render(" (default)")
		}
		if i == m.pickerIdx {
			b.WriteString(selStyle.Render("❯ "+w.Name) + tag + "  " + faintStyle.Render(w.Slug) + "\n")
		} else {
			b.WriteString("  " + w.Name + tag + "  " + faintStyle.Render(w.Slug) + "\n")
		}
	}
	b.WriteString("\n" + faintStyle.Render("↑/↓ select · enter confirm · esc cancel"))
	m.vp.SetContent(b.String())
	m.vp.GotoTop()
}

func (m *replModel) refreshPRPicker() {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Select a pull request") + "\n\n")
	for i, pr := range m.prItems {
		num := fmt.Sprintf("#%-5d", pr.PRNumber)
		title := pr.PRTitle
		if title == "" {
			title = pr.PRURL
		}
		status := faintStyle.Render("  " + prStatusLabel(pr.Status))
		if !pr.Ready() {
			status = faintStyle.Render("  " + prStatusLabel(pr.Status) + " — not ready")
		}
		if i == m.prIdx {
			b.WriteString(selStyle.Render("❯ "+num+" "+title) + status + "\n")
		} else {
			b.WriteString("  " + num + " " + title + status + "\n")
		}
	}
	b.WriteString("\n" + faintStyle.Render("↑/↓ select · enter confirm · esc cancel"))
	m.vp.SetContent(b.String())
	m.vp.GotoTop()
}

func (m *replModel) refreshChatPicker() {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Resume a conversation") + "\n\n")
	for i, chat := range m.chatItems {
		line := chatLabel(chat)
		when := faintStyle.Render("  " + chatWhen(chat))
		if i == m.chatIdx {
			b.WriteString(selStyle.Render("❯ "+line) + when + "\n")
		} else {
			b.WriteString("  " + line + when + "\n")
		}
	}
	b.WriteString("\n" + faintStyle.Render("↑/↓ select · enter confirm · esc cancel · /new starts a fresh one"))
	m.vp.SetContent(b.String())
	m.vp.GotoTop()
}

// chatLabel names a conversation in the picker: its title, prefixed with the pull
// request it is scoped to. A conversation the backend hasn't summarized yet may have
// no title, so its slug stands in — it is still resumable.
func chatLabel(chat apiclient.Chat) string {
	title := strings.TrimSpace(chat.Title)
	if title == "" {
		title = shortSlug(chat.Slug)
	}
	if chat.IsPR() && chat.PRNumber != "" {
		return "#" + chat.PRNumber + " · " + title
	}
	return title
}

// chatWhen renders when a conversation started, in local time. An unparseable
// timestamp is left out rather than guessed at.
func chatWhen(chat apiclient.Chat) string {
	started, err := time.Parse(time.RFC3339, chat.CreatedAt)
	if err != nil {
		return ""
	}
	return started.Local().Format("2006-01-02 15:04")
}

func (m *replModel) statusBar() string {
	ws := m.wsName
	if ws == "" {
		ws = shortSlug(m.env.workspace)
	}
	pr := m.activePR
	if pr == "" {
		pr = "—"
	}
	return labelStyle.Render("⬡ workspace ") + ws + labelStyle.Render("    ⎇ PR ") + pr
}

// rule draws a full-width horizontal divider framing the input box.
func (m *replModel) rule() string {
	return faintStyle.Render(strings.Repeat("─", m.width))
}

// View renders the frame, along with the terminal features it needs: a
// full-screen transcript and mouse reporting for scrolling it.
func (m *replModel) View() tea.View {
	v := tea.NewView(m.content())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *replModel) content() string {
	if !m.ready {
		return "loading…"
	}
	if m.mode == modePicking {
		return m.vp.View() + "\n" + faintStyle.Render("selecting workspace…") + "\n\n" + m.statusBar()
	}
	if m.mode == modePickingPR {
		return m.vp.View() + "\n" + faintStyle.Render("selecting pull request…") + "\n\n" + m.statusBar()
	}
	if m.mode == modePickingChat {
		return m.vp.View() + "\n" + faintStyle.Render("selecting conversation…") + "\n\n" + m.statusBar()
	}

	// The animated status/spinner lives in the transcript itself (see
	// spinnerLine), so this hint stays static regardless of streaming state.
	hint := faintStyle.Render("enter send · ↑↓ history · ctrl+r resume · pgup/pgdn/mouse scroll · drag to copy · esc cancel · ctrl+c clear · ctrl+d quit")
	if m.copied {
		hint = faintStyle.Render("copied the selection to the clipboard · esc clears it")
	}
	// Rules frame the input box, and the status bar sits at the very bottom.
	rule := m.rule()
	return m.vp.View() + "\n" + hint + "\n" + rule + "\n" + m.input.View() + "\n" + rule + "\n" + m.statusBar()
}

const replHelp = `commands:
  /chats             resume an earlier conversation (also ctrl+r)
  /pr                open the pull request picker
  /pr <url>          scope to a pull request URL (must be a completed review)
  /pr clear          remove the PR scope
  /workspace [slug]  switch workspace (no arg = picker; validates access)
  /new               start a fresh conversation
  /help              show this help
  /exit              quit (or press ctrl+d)

Conversations are shared with the web app: what you start here is listed there,
and what you start there can be resumed here.`

func workspaceLabel(name, slug string) string {
	if name == "" {
		return slug
	}
	return name + " (" + shortSlug(slug) + ")"
}

func shortSlug(slug string) string {
	if len(slug) <= 8 {
		return slug
	}
	return slug[:8] + "…"
}

func isCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return strings.Contains(err.Error(), "context canceled")
}
