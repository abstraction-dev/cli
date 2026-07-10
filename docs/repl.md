# Interactive Mode (REPL)

Confirmed all the slash commands and the entry function. Now let me check how the REPL session is started (the `ask` command / interactive mode entry point) and confirm the CLI command dispatcher connects to `runREPL`.Good, confirms interactive mode is triggered by running `abstr ask` (or the ask command) with no question text and a real terminal attached. Now I have enough grounding to write the guide.# Interactive REPL Mode

Astrid's interactive REPL lets you have an ongoing, conversational chat with Astrid right in your terminal instead of asking one-off questions.

## Starting a session

Run `abstr ask` with no question text in a real terminal, and the CLI drops you into the interactive chat loop instead of a single-shot answer, as decided by [resolveQuery](internal/cli/ask.go) and launched by [runAsk](internal/cli/ask.go). Before your first message appears, [runREPL](internal/cli/repl.go) creates a fresh session ID, prepares the input box, and prints a welcome line reminding you of `/help` and `/exit`. Make sure you've completed [getting started](index.md) and are [logged in](login.md) first.

## Chatting

Just type your question and press enter to ask Astrid, as described in [Asking Questions](ask.md). You can also type slash commands, which are routed by [runCommand](internal/cli/repl.go):

- `/help` — show available commands
- `/new` — start a fresh conversation
- `/workspace` — switch or pick a [workspace](workspaces.md)
- `/pr` — attach a pull request to your conversation

See the full [command reference](reference.md) for details.

## Exiting

Type `/exit` or `/quit` at any time to end the session, both of which are [runCommand](internal/cli/repl.go) and immediately close the REPL.

