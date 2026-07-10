# Interactive Mode (REPL)

# Interactive REPL Mode

Use the CLI's interactive REPL to have a back-and-forth chat with Astrid in a single terminal session.

## Starting a session

Run the [runAsk](internal/cli/ask.go) without typing a question, and when your input is a live terminal, [resolveQuery](internal/cli/ask.go) switches automatically into interactive mode. From there, [runREPL](internal/cli/repl.go) creates a session id, detects your terminal's background for markdown styling, and opens a full-screen chat window seeded with a welcome message telling you to type /help for commands or /exit to quit.


```mermaid
flowchart TD
    N1["func runREPL(env *appEnv, initialPR string) int"]
    N2["RETURN_NODE"]
    N3["return exitOK"]
    N4["Run the terminal session with failure handling"]
    N8["m := &replModel{
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
    N9["ti.Placeholder = 'Ask Astrid…  (/help, /exit)'
ti.ShowLineNumbers = false
ti.CharLimit = 8192
ti.SetHeight(inputHeight)
ti.Focus()"]
    N10["Keep the input area visually plain"]
    N12["ti.FocusedStyle.Prompt = userStyle
ti.BlurredStyle.Prompt = userStyle"]
    N13["Show the prompt only on the first line"]
    N15["sessionID, _ := uuidutil.New()
md := render.NewMDRenderer(render.HasDarkBackground(), 0)
ti := textarea.New()
ti.Prompt = inputPrompt"]
    N3 -->|RETURN_STEP| N2
    N4 -->|RETURN_STEP| N2
    N4 -->|BRANCH_FALSE_CASE| N3
    N8 -->|COMPUTE_STEP| N4
    N9 -->|COMPUTE_STEP| N8
    N10 -->|COMPUTE_STEP| N9
    N12 -->|COMPUTE_STEP| N10
    N13 -->|COMPUTE_STEP| N12
    N15 -->|COMPUTE_STEP| N13
    N1 -->|COMPUTE_STEP| N15

```



## Chatting

Type your question and press enter to send it. Slash commands are handled by [runCommand](internal/cli/repl.go): /help shows the built-in help text, /new (or /reset) starts a fresh conversation, /workspace (or /ws) opens the workspace picker or switches directly when you give a name, and /pr opens the pull request picker, clears the active PR with `/pr clear`, or sets one directly. See the [command reference](reference.md) for the full list, and [managing workspaces](workspaces.md) for workspace switching details.

## Exiting

Type /exit or /quit at any time, and [runCommand](internal/cli/repl.go) ends the session right away.

See also [getting started](index.md), [logging in](login.md), and [asking questions](ask.md).

