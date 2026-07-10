# Getting Started

## Installing the CLI

The Abstraction CLI, called `abstr`, is a compiled Go binary that gives you [CLI Executable (abstr)](concept/cacc52d2-ad0c-430b-a569-5779139301ed). Once it's installed on your machine, you run it from your terminal by typing `abstr` followed by whatever you want to do.

## Running It for the First Time

If you just type `abstr` with nothing else, the CLI does not simply print an error — [Main](internal/cli/root.go) checks whether you gave it a specific subcommand like `login`, `workspace`, or `config`, and if not, it falls through to the [runAsk](internal/cli/ask.go). When there's no question typed and no piped input, and you're sitting at an interactive terminal, `abstr` [resolveQuery](internal/cli/ask.go) rather than asking you to add extra flags.

If you'd like to see what the tool can do before diving in, running `abstr help` (or `-h` / `--help`) prints a [printUsage](internal/cli/root.go) covering every command, flag, and interactive shortcut.

## What to Do Next

Before you can ask anything, the CLI needs an API key and a workspace to talk to. Running `abstr login` walks you through this: it [Load](internal/config/config.go), and if no API key is set yet, it [bootstrapAPIKey](internal/cli/env.go). See login.md for the full walkthrough.

Once you're logged in, you're ready to start asking questions — either by typing one directly after `abstr`, piping text into it, or just staying in the interactive session that opens when you run `abstr` on its own. See ask.md for details on asking questions, and config.md if you ever need to check or change where your configuration is stored.

