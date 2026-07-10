# Interactive Mode (REPL)

## Interactive REPL Mode

The CLI's interactive mode turns the tool into a running chat session with Astrid, instead of answering a single question and exiting. This mode is driven by the [runREPL](internal/cli/repl.go) flow, which sets up the session and hands control to a full-screen terminal program.

### Starting an interactive session

If you run the ask command without typing a question and without piping any text in, the CLI treats that as a request to chat interactively — this decision is made by [resolveQuery](internal/cli/ask.go), which falls back to interactive mode whenever no question was given directly and your terminal is attached interactively. Once your workspace and credentials are ready, [runAsk](internal/cli/ask.go) hands off to [runREPL](internal/cli/repl.go) to actually start the session.

When the session starts, [runREPL](internal/cli/repl.go) creates a unique session identifier, prepares a markdown renderer that adapts to your terminal's light or dark background, and sets up the input box where you'll type your messages. It then opens a full-screen chat view and greets you with a short welcome message reminding you that `/help` lists commands and `/exit` quits.

If the interactive screen fails to start for some reason, the CLI reports the error and exits with a failure status; otherwise it runs until you choose to leave.

See [Getting Started](getting-started.md) for how to configure the CLI before your first session, and [Logging In](login.md) if you haven't authenticated yet.

### Having a multi-turn conversation

Once the session is open, you type your question into the input box and press Enter to send it. Submitting a message is handled by [submit](internal/cli/repl.go), which cleans up your text, saves it to your input history, and — if it isn't a slash command — adds it to the visible conversation and starts a new answer cycle through [startTurn](internal/cli/repl.go).

[startTurn](internal/cli/repl.go) sends your question to the backend and streams the answer back progressively, so you see the response appear as it's generated rather than waiting for the whole thing at once. Each new question you ask becomes another turn in the same conversation, keeping the context of your session.

While a response is streaming, typing is paused so you can't accidentally edit your draft, and pressing Enter is ignored until the current answer finishes, as handled by [handleKey](internal/cli/repl.go). You can recall previous messages you've typed using the up and down arrow keys, which move through your input history via the REPL's [handleKey](internal/cli/repl.go) navigation logic.

If you want to start over, typing `/new` or `/reset` clears the current conversation and begins a fresh one, and you can switch which workspace or pull request the conversation is scoped to with `/workspace` and `/pr`, both handled by [runCommand](internal/cli/repl.go). See [Managing Workspaces](workspace.md) for more on workspace and PR scoping, and [Command Reference](command-reference.md) for the full list of slash commands. For details on how questions and answers work outside of interactive mode, see [Asking Questions](ask.md).

### Exiting the session

To leave the interactive session, type `/exit` or `/quit` and press Enter — this is handled directly by [runCommand](internal/cli/repl.go), which ends the chat program. You can also press Ctrl+D while the session is idle and your input box is empty, which quits immediately via [handleKey](internal/cli/repl.go). Pressing Ctrl+C or Esc while an answer is still streaming instead cancels that in-progress response rather than closing the session, letting you keep chatting afterward.

When the session ends normally, the CLI exits cleanly; if the interactive program itself failed to run, it reports the error before exiting instead.

