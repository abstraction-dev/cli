# Interactive Mode (REPL)

# Interactive REPL Mode

Astrid's interactive REPL lets you chat with Astrid, switch workspaces, and scope your questions to a pull request, all from one continuous terminal session.

## Starting a Session

Launching interactive mode hands control to the [runREPL](internal/cli/repl.go), which creates a new session identifier, prepares the markdown-aware terminal renderer, and configures the text input area before launching the full-screen Bubble Tea program. The session opens with a welcome message reminding you to type `/help` for commands or `/exit` to quit. If you're not sure how to get set up first, see the [Getting Started](index.md) guide, and make sure you're [logged in](login.md) beforehand.

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

Once the session is running, just type your question and press enter to ask Astrid — see [Asking Questions](ask.md) for tips on getting good answers. You can also type slash commands, which are interpreted by the [runCommand](internal/cli/repl.go): `/help` shows the built-in help text, `/new` (or `/reset`) starts a fresh conversation, `/workspace` (or `/ws`) opens a workspace picker or switches directly to a named workspace, and `/pr` opens a pull request picker, sets a specific PR, or clears the active PR scope with `/pr clear`. Full details on workspaces are in [Managing Workspaces](workspaces.md), and every command is listed in the [Command Reference](reference.md).

## Exiting

Type `/exit` or `/quit` at any time to end the session, which the [runCommand](internal/cli/repl.go) handles by quitting the Bubble Tea program cleanly.

