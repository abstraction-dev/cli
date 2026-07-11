# Managing Workspaces

# Managing Workspaces

Use the `workspace` command family to inspect or change which workspace your CLI commands run against, handled by [runWorkspace](internal/cli/workspace.go).

```
abstr workspace [list|use|pick]
```

## Listing workspaces

Run `abstr workspace list` to fetch all workspaces tied to your account and print each one's slug and name, marking your current workspace with a `*` marker.

```
abstr workspace list
```

## Picking one

If you don't specify a subcommand, the CLI defaults to `pick`, which lets [pickWorkspace](internal/cli/env.go) select your workspace automatically when there's only one, or show a numbered list and prompt you to choose when there are several.

```
abstr workspace
```

Type the number shown next to your choice — invalid entries are rejected and you'll be asked again.

## Switching workspace

Run `abstr workspace use <slug|name>` to switch directly, which matches your input against workspace slugs and names (case-insensitive) before saving the new selection.

```
abstr workspace use my-team
```

You'll need to be [logged in](login.md) first. For all available flags and subcommands, see the [command reference](reference.md).

