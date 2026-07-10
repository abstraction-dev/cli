# Managing Workspaces

# Managing Workspaces from the CLI

Workspaces let you keep your data and settings separated — for example, one workspace for personal projects and another for a team. The CLI gives you three ways to work with them: listing what's available, letting the tool pick one for you, and switching to a specific one. All of this is handled by the `abstr workspace` (or its shorthand `abstr ws`) command, which is wired up in the CLI's [Main](internal/cli/root.go).

Before using workspace commands, you'll need to be logged in — see the [login guide](../login.md) if you haven't done that yet. The workspace command checks for a saved API key and, if you're not logged in, it stops and tells you to run `abstr login` first, as part of [runWorkspace](internal/cli/workspace.go).

## Listing available workspaces

Run:

```
abstr workspace list
```

This fetches every workspace your account can access and prints each one's short identifier and display name, marking the one you're currently using with a `*` next to it, all handled by the `list` branch of [runWorkspace](internal/cli/workspace.go). The list of workspaces itself comes from a request to the workspaces API, made through the [Workspaces](internal/apiclient/client.go), and the "currently active" workspace is determined by [WorkspaceResolved](internal/config/config.go), which prefers an environment override over your saved configuration.

## Picking a workspace

If you don't specify what to do, or you just run:

```
abstr workspace
```

the CLI will pick a workspace for you interactively, through [pickWorkspace](internal/cli/env.go). If your account only has one workspace, it's chosen automatically and you're told which one is being used. If you have several, you'll see a numbered list — with the account's default workspace marked `(default)` — and you'll be prompted to type a number; the tool will keep asking if you type something invalid. If your account has no workspaces at all, you'll get a clear error saying none were found. Whatever you pick, the choice is then saved to your configuration by [saveWorkspace](internal/cli/workspace.go).

## Switching workspace

To switch directly to a known workspace, run:

```
abstr workspace use <slug or name>
```

You can type either the workspace's short identifier or its full name (multi-word names don't need quotes), and the command will match it case-insensitively against the available list before saving it as your active workspace — this whole path is the `use` branch of [runWorkspace](internal/cli/workspace.go). If nothing matches, you'll get an error telling you no workspace matched what you typed.

In every case — picking or switching — the result is written to your configuration file by [saveWorkspace](internal/cli/workspace.go), which also rolls back to your previous workspace if saving to disk fails, so you're never left in an inconsistent state.

For the full list of workspace command options and other CLI commands, see the [command reference](../reference.md).

