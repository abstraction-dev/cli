# Getting Started

## Getting Started with the CLI

Welcome! This guide walks you through installing the CLI, running it for the first time, and taking your first steps once it's set up.

### 1. Install the CLI

The command-line tool is called `abstr`. Once installed, running it without any arguments — or with a question typed after it — is enough to get started, since the tool [Main](internal/cli/root.go) straight to its default question-answering flow if you don't type a specific command.

### 2. Run it for the first time

Just type `abstr` in your terminal. If you don't provide a question and your terminal is interactive, the CLI automatically opens a live conversation session instead of exiting immediately — this decision about whether to go interactive, read piped input, or use text typed directly after the command is made automatically for you every time you run it. This behavior is 

You don't need to type any special flag to get into this conversation mode — it's the default when nothing else is provided, and it hands off to an [runREPL](internal/cli/repl.go) that keeps a running back-and-forth with Astrid until you exit.

The first time you run a command, if you haven't stored an API key yet, the CLI will notice and walk you through getting one: it opens your browser to the settings page, asks you to paste in your key, and checks that it works — giving you a few tries if you mistype it. This first-time setup is [bootstrapAPIKey](internal/cli/env.go) so you don't need to look anything up yourself.

### 3. What to do next

**Log in.** Before asking real questions, run `abstr login` to store your API key and choose your workspace. This command [runLogin](internal/cli/login.go) so you don't have to repeat the setup next time. See the full [login guide](../login.md) for details.

**Ask a question.** Once you're logged in, just run `abstr` followed by your question, or start it with no question to chat interactively. Full usage details — including flags for workspaces, pull requests, and more — are covered in the [ask guide](../ask.md).

**Check your configuration.** If you ever want to see where your settings are stored or confirm what's active, run `abstr config show` or `abstr config path`. See the [configuration guide](../config.md) for more.

That's it — install, run once to get set up, then log in and start asking questions.

