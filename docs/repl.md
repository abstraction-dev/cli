# Interactive Mode (REPL)

## Using the Interactive REPL Mode

The CLI's interactive mode lets you have an ongoing, multi-turn conversation with Astrid in your terminal instead of asking one question at a time. This is powered by the [runREPL](internal/cli/repl.go) function, which sets up and runs the whole session.

### Starting an interactive session

When you launch the interactive mode, [runREPL](internal/cli/repl.go) prepares everything you need for a chat session: it creates a unique session identifier for your conversation, detects whether your terminal uses a light or dark background so answers are readable, and sets up the text box where you'll type your messages. It then displays a welcome message letting you know you're "Chatting with Astrid" and reminding you that `/help` lists available commands and `/exit` quits, before handing control over to the full-screen terminal interface.

### Having a multi-turn conversation

Once the session is running, anything you type and submit is handled by [submit](internal/cli/repl.go): plain text is added to the conversation transcript and sent off as a new question via [startTurn](internal/cli/repl.go), which streams the answer back to your screen as it's generated. When a response finishes — whether it completed normally, was cancelled, or hit an error — [finishTurn](internal/cli/repl.go) records the outcome in the transcript and hands control back to the input box so you can ask a follow-up. Because each message in the session shares the same conversation, you can ask follow-up questions that build on earlier answers without repeating context. If you type a message starting with `/`, it's treated as a command instead of a question — for example, `/new` or `/reset` starts a fresh conversation, `/workspace` (or `/ws`) switches your active workspace, and `/pr` selects a pull request to focus on, all handled by [runCommand](internal/cli/repl.go). For more on how individual questions are answered, see the [asking questions guide](../ask.md).

### Exiting the session

To leave the interactive session, type `/exit` or `/quit`. This command is caught by [runCommand](internal/cli/repl.go), which ends the session cleanly and returns you to your regular terminal prompt.

