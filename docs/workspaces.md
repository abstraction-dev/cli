# Managing Workspaces

## Managing Workspaces from the CLI

Workspaces let you separate your questions, settings, and data by team or project. The `abstr workspace` command is your control center for switching between them, and everything below is handled by the [runWorkspace](internal/cli/workspace.go), which loads your configuration, confirms you're logged in, and then carries out whichever workspace action you ask for.

If you haven't signed in yet, this command will stop and tell you to log in first — see login.md for how to do that.

### Listing available workspaces

To see every workspace available to your account, run `abstr workspace list`. This asks the dispatcher to fetch [Workspaces](internal/apiclient/client.go) from your account and print each one on its own line, with the workspace you're currently using marked by a `*` next to its name.

### Picking a workspace

If you just run `abstr workspace` with no extra words, it defaults to helping you pick one interactively through [pickWorkspace](internal/cli/env.go). If your account only has a single workspace, it's selected for you automatically and you'll see a short confirmation message. If you have more than one, you'll be shown a numbered list — with the default workspace labeled — and asked to type the number of the one you want. If you type something that isn't a valid number on the list, you'll get a warning and be asked to try again, so there's no risk of accidentally picking the wrong thing.

### Switching workspace directly

If you already know the workspace you want, you can jump straight to it with `abstr workspace use <name-or-slug>`. This looks through your available workspaces for one whose short identifier matches exactly, or whose full name matches what you typed (capitalization doesn't matter). You can type a multi-word name without quotes and it will still be matched correctly. If nothing matches, you'll be told plainly that no workspace matched what you entered.

### Saving your choice

Whether you picked a workspace interactively or switched with `use`, the result is handed off to [saveWorkspace](internal/cli/workspace.go), which writes your new workspace choice to your configuration file so it's remembered the next time you run a command. If saving fails for any reason, your previous workspace setting is kept in place rather than left in a broken state, and you'll see an error message explaining what went wrong.

For the complete list of workspace-related commands and their options, see reference.md.

