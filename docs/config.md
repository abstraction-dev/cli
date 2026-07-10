# Configuration

# Config Command

The `config` command lets you inspect the CLI's current configuration without editing files by hand. It is handled by the [runConfig](internal/cli/configcmd.go).

## Subcommands

### `show` (default)

Running `abstr config` or `abstr config show` prints a summary of the active configuration: the config file path, the workspace, a masked API key, and the resolved API URL — all produced by the [runConfig](internal/cli/configcmd.go).

### `path`

Running `abstr config path` prints only the config file location, using the [FilePath](internal/config/config.go).

If no subcommand is given, `show` is used as the default action.

## Flags

- `--config <path>` — points the command at an alternate config file instead of the default location resolved by [Load](internal/config/config.go).

## Masked values

The API key is never printed in full. It is passed through a [redactKey](internal/cli/configcmd.go) that shows `(not set)` when empty, `****` for short keys, or the first 8 characters followed by an ellipsis for longer keys. The API URL shown by `show` is the [BaseURLResolved](internal/config/config.go), which prefers an environment override, then the configured value, and finally falls back to the default.

Here's the flow through the command when it runs

```mermaid
flowchart TD
    N1["func runConfig(args []string) int"]
    N2["RETURN_NODE"]
    N3["return exitOK"]
    N4["switch action"]
    N5["Reject unknown action"]
    N8["Print configuration summary"]
    N10["Print config file path"]
    N12["Pick explicit action"]
    N15["action := 'show'"]
    N16["Handle configuration load errors"]
    N20["Read command and load configuration"]
    N22["Parse arguments"]
    N25["Initialize config command flags"]
    N3 -->|RETURN_STEP| N2
    N5 -->|RETURN_STEP| N2
    N4 -->|case | N5
    N8 -->|COMPUTE_STEP| N3
    N4 -->|case "show"| N8
    N10 -->|COMPUTE_STEP| N3
    N4 -->|case "path"| N10
    N12 -->|COMPUTE_STEP| N4
    N12 -->|BRANCH_FALSE_CASE| N4
    N15 -->|COMPUTE_STEP| N12
    N16 -->|RETURN_STEP| N2
    N16 -->|BRANCH_FALSE_CASE| N15
    N20 -->|COMPUTE_STEP| N16
    N22 -->|RETURN_STEP| N2
    N22 -->|BRANCH_FALSE_CASE| N20
    N25 -->|COMPUTE_STEP| N22
    N1 -->|COMPUTE_STEP| N25

```



## See also

- [Getting Started](../getting-started.md)
- [Login & Authentication](../login-authentication.md)
- [Asking Questions](../asking-questions.md)
- [Managing Workspaces](../managing-workspaces.md)
- [Command Reference](../command-reference.md)

