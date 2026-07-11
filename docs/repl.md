# Interactive Mode (REPL)

## What Interactive Mode Is

Interactive mode turns the command-line tool into a live, ongoing conversation with Astrid instead of a single question-and-answer exchange. Rather than typing one question, waiting for a reply, and starting the command over again, you open a persistent chat session where you can ask follow-up questions, change context, and keep the discussion going — all without leaving the terminal.

If you run the tool without typing a question, and you're working directly in a terminal window (rather than piping input from another program), it opens this interactive session automatically. If you already know what you want to ask, you can still just type your question directly and get a single answer — see [Asking Questions](asking-questions.md) for that simpler flow.

## What It Feels Like

Once inside, the screen becomes a dedicated chat view. A status line at the top always shows which workspace you're currently working in and, if you've scoped the conversation to a specific pull request, which one that is. Below that is the running transcript of your conversation, and at the bottom is a text box where you type.

When you ask something, the answer streams in progressively — you see it being written in real time rather than waiting for the whole response to arrive at once — with a status indicator letting you know a reply is being generated. Answers are formatted for easy reading, with headings, lists, and emphasis rendered cleanly rather than as raw text.

You can scroll back through earlier parts of the conversation at any time, and the input box remembers what you've typed before, so pressing the up and down arrows recalls earlier questions the way command-line history normally works.

## What You Can Do Inside

While chatting normally, you can also type a small set of special commands (each starting with `/`) to control the session itself:

- **Start a new conversation** — `/new` or `/reset` clears the current thread and begins fresh, useful when you want to change topics without old context carrying over.
- **Switch workspaces** — `/workspace` (or `/ws`) on its own opens a list of your available workspaces to choose from; adding a workspace name switches directly to it. See [Managing Workspaces](managing-workspaces.md) for more on what a workspace represents.
- **Scope the conversation to a pull request** — `/pr` opens a list of pull requests you can pick from; `/pr` followed by a GitHub pull request link scopes the conversation to that specific PR; `/pr clear` removes that scope. If a link doesn't look like a valid GitHub PR URL, or points to a PR that isn't ready yet, you're told clearly what went wrong instead of the request silently failing.
- **See available commands** — `/help` prints a quick reminder of everything above.
- **Leave the session** — `/exit` or `/quit` ends the conversation and returns you to your regular terminal. Pressing Ctrl+D while the input box is empty does the same thing.

Switching workspaces or changing the pull request scope automatically starts a new conversation thread, since the context you're asking about has changed. If a workspace switch can't be saved for some reason, the session keeps your previous workspace active rather than leaving things in a half-changed state, and lets you know what happened.

## Staying In Control While an Answer Is Streaming

If a response is still being generated and you'd rather not wait for it, pressing Ctrl+C or Esc cancels it partway through. The transcript notes that the reply was cancelled, and you're free to ask something else immediately. While an answer is streaming, the input box is locked from editing so you don't accidentally start typing into a response mid-flight — normal typing resumes as soon as the current turn finishes.

## Getting Set Up

Interactive mode uses the same workspace and account setup as the rest of the tool. If you haven't logged in or chosen a workspace yet, the tool walks you through that before the chat session opens — see [Logging In](logging-in.md) for details on connecting your account, and [Configuration](configuration.md) for how your settings (like your default workspace or API address) are stored and reused between sessions.

For a broader overview of everything the tool can do — both inside and outside interactive mode — see the [Command Reference](command-reference.md), or start from [Getting Started](getting-started.md) if you're setting things up for the first time.

