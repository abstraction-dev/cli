# Interactive Mode (REPL)

# Interactive REPL Mode

The `abstr` CLI includes a live chat-style session for asking multiple questions in a row without re-running the command each time. This flow is driven by the [runREPL](internal/cli/repl.go), which shows how a session is created, chatted in, and exited

```mermaid
flowchart TD
    N1["func runREPL(env *appEnv, initialPR string) int"]
    N2["RETURN_NODE"]
    N3["func (m *replModel) submit() (tea.Model, tea.Cmd)"]
    N4["RETURN_NODE"]
    N5["func (m *replModel) runCommand(cmd string) (tea.Model, tea.Cmd)"]
    N6["RETURN_NODE"]
    N7["return exitOK"]
    N8["Run the terminal session with failure handling"]
    N12["m := &replModel{
    env: env,
    sub: make(chan tea.Msg, 256),
    md: md,
    input: ti,
    sessionID: sessionID,
    activePR: initialPR,
}
m.entries = []transcriptEntry{transcriptEntry{
    entrySystem,
    'Chatting with Astrid. Type /help for commands, /exit to quit.',
}}"]
    N13["ti.Placeholder = 'Ask Astrid…  (/help, /exit)'
ti.ShowLineNumbers = false
ti.CharLimit = 8192
ti.SetHeight(inputHeight)
ti.Focus()"]
    N14["Keep the input area visually plain"]
    N16["ti.FocusedStyle.Prompt = userStyle
ti.BlurredStyle.Prompt = userStyle"]
    N17["Show the prompt only on the first line"]
    N19["sessionID, _ := uuidutil.New()
md := render.NewMDRenderer(render.HasDarkBackground(), 0)
ti := textarea.New()
ti.Prompt = inputPrompt"]
    N20["Submit normal chat input"]
    N23["Command prefix guard"]
    N25["return m.runCommand(q)"]
    N26["Record submission and prepare the REPL"]
    N28["Empty input guard"]
    N30["return m, nil"]
    N31["q := strings.TrimSpace(escLeakPattern.ReplaceAllString(m.input.Value(), ''))"]
    N32["Finish command processing"]
    N34["Execute the matched slash command"]
    N57["fields := strings.Fields(cmd)"]
    N7 -->|RETURN_STEP| N2
    N8 -->|RETURN_STEP| N2
    N8 -->|BRANCH_FALSE_CASE| N7
    N12 -->|COMPUTE_STEP| N8
    N13 -->|COMPUTE_STEP| N12
    N14 -->|COMPUTE_STEP| N13
    N16 -->|COMPUTE_STEP| N14
    N17 -->|COMPUTE_STEP| N16
    N19 -->|COMPUTE_STEP| N17
    N1 -->|COMPUTE_STEP| N19
    N20 -->|RETURN_STEP| N4
    N25 -->|RETURN_STEP| N4
    N23 -->|BRANCH_TRUE_CASE| N25
    N23 -->|BRANCH_FALSE_CASE| N20
    N26 -->|COMPUTE_STEP| N23
    N30 -->|RETURN_STEP| N4
    N28 -->|BRANCH_TRUE_CASE| N30
    N28 -->|BRANCH_FALSE_CASE| N26
    N31 -->|COMPUTE_STEP| N28
    N3 -->|COMPUTE_STEP| N31
    N32 -->|RETURN_STEP| N6
    N34 -->|COMPUTE_STEP| N32
    N34 -->|RETURN_STEP| N6
    N57 -->|COMPUTE_STEP| N34
    N5 -->|COMPUTE_STEP| N57

```

.

## Starting a session

A session begins when the [runREPL](internal/cli/repl.go) generates a unique session ID, prepares the markdown-styled output area, and shows a welcome message reminding you to type `/help` for commands or `/exit` to quit. Before you get here, make sure you've completed [logging in](login.md) and reviewed [getting started](index.md).

## Chatting

Type a question and press Enter to send it — plain text is added to the conversation and answered, while anything starting with `/` is treated as a command by the [runCommand](internal/cli/repl.go). Supported commands include `/help` (show help), `/new` (start a fresh conversation), `/workspace` (switch or pick a [workspace](workspaces.md)), and `/pr` (attach a pull request to the conversation, or clear it with `/pr clear`) — see the full [command reference](reference.md) and [asking questions](ask.md) guide for details.

## Exiting

Type `/exit` or `/quit` at any time to end the session — the [runCommand](internal/cli/repl.go) recognizes both and quits the REPL immediately.

