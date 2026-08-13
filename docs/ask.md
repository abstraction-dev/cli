# Asking Questions

## Asking Questions

Asking a question is the core thing this tool is for: you type (or paste) a question about your codebase, and the tool sends it to your workspace's AI agent, then prints back an answer directly in your terminal.

### How you ask

There are three ways to get a question to the tool:

* **Type it directly on the command line**, as the text you pass when running the tool. This is the fastest way to ask something quick — for example, running the tool followed by your question in quotes.

* **Pipe text into it** from another program or file. If you don't type a question directly, the tool checks whether anything has been piped in and uses that text as the question instead.

* **Leave it blank and answer interactively.** If you don't provide a question and nothing is piped in, and you're running the tool in a normal terminal window, it opens a live back-and-forth conversation instead of a single question-and-answer exchange. That experience is covered separately in [Interactive Mode (REPL)](interactive-mode-repl.md).

If none of these produce a usable question — for example, a script piped in nothing but blank lines — the tool stops and tells you no question was provided rather than guessing.

You can also point a question at a specific pull request by supplying a GitHub pull request link alongside your question. The tool checks that the value looks like a genuine GitHub pull request URL before sending anything, and rejects it up front with a clear message if it doesn't.

### What happens before your question is sent

Before your question ever reaches the server, the tool makes sure it has everything it needs to send it on your behalf:

* It loads your saved settings (API connection details and, if you have one, a default workspace). See [Configuration](configuration.md) for what's stored and where.

* If you haven't logged in yet or don't have an API key on file, it walks you through connecting your account first — see [Logging In](logging-in.md).

* If no workspace is set — either saved in your configuration or passed in as a flag — and you're asking a single question (not interactively), the tool stops and asks you to specify one or log in, rather than guessing which workspace you meant. In interactive mode, it instead lets you pick a workspace on the spot. More on workspaces in [Managing Workspaces](managing-workspaces.md).

Only once a workspace and a valid connection are confirmed does the tool actually send your question.

### What happens when your question is sent

Your question, along with your workspace, any linked pull request, and a request to receive one final answer, is sent securely to your workspace's agent. The tool waits for that agent to finish working and return a single, complete answer — there's no partial or live output in this single-question mode (that behavior belongs to interactive mode, where answers can stream in as they're generated).

If you press Ctrl+C while a question is in progress, the tool cancels the request cleanly and reports that the run was interrupted, rather than showing a confusing error.

If something goes wrong — a network problem, an authentication failure, or the agent reporting an error — the tool prints a clear, human-readable error message describing what happened, and exits with a status code that reflects the type of failure. This makes it easy to tell an ordinary hiccup from something like a missing configuration.

### What you get back

Once the answer arrives, the tool decides how to display it based on how you're using it:

* If you're looking at your terminal directly, the answer is rendered as nicely formatted text — headings, lists, code blocks, and emphasis all display the way you'd expect, sized to fit your terminal window.

* If the tool's output is being redirected or piped into another program (for example, saved to a file or passed to another command), it prints the plain, unformatted answer instead, so downstream tools receive clean, predictable text rather than terminal styling codes.

Either way, you get the complete answer text once the agent has finished, and the command then exits.

### Related pages

* [Getting Started](getting-started.md)

* [Command Reference](command-reference.md)

* [Logging In](logging-in.md)

* [Managing Workspaces](managing-workspaces.md)

* [Configuration](configuration.md)

* [Interactive Mode (REPL)](interactive-mode-repl.md)



