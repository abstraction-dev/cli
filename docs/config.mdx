# Configuration

Configuration reference for the `abstr config` command, covering how to view or locate your active settings.

## Overview

The `config` command is handled by [runConfig](internal/cli/configcmd.go), which loads settings and then prints either the config path or a redacted configuration summary depending on the subcommand given. It is reached when you run `abstr config ...`, since [Main](internal/cli/root.go) routes the `"config"` argument to this handler.

## Subcommands

### `show` (default)

Running `abstr config` or `abstr config show` prints a four-line summary: the config file path, the active workspace, a masked API key, and the resolved API URL, all produced by [runConfig](internal/cli/configcmd.go).

* **config** — the file path returned by [FilePath](internal/config/config.go).

* **workspace** — the workspace value stored in the loaded configuration.

* **api\_key** — the API key, masked by [redactKey](internal/cli/configcmd.go), which shows `(not set)` when empty, `****` for short keys, or the first 8 characters followed by an ellipsis otherwise.

* **api\_url** — the effective API URL as computed by [BaseURLResolved](internal/config/config.go), which prefers an environment override, falls back to the configured URL, and finally to the built-in default.

### `path`

Running `abstr config path` prints only the config file location, again via [runConfig](internal/cli/configcmd.go) calling [FilePath](internal/config/config.go). Use this when you just need the file's location, for example to open it in an editor.

## The `--config` flag

Both subcommands accept a `--config <file>` flag to point at an alternate config file instead of the default location, since [runConfig](internal/cli/configcmd.go) parses this flag and passes it through when loading settings. If loading fails, the command reports an error and exits.

## Notes

* Sensitive values such as the API key are never printed in full — they are always masked by [redactKey](internal/cli/configcmd.go) in `show` output.

* An unrecognized subcommand results in a usage error, as handled by [runConfig](internal/cli/configcmd.go).

## See also

* [Getting Started](../getting-started.md)

* [Login & Authentication](../login-authentication.md)

* [Managing Workspaces](../managing-workspaces.md)

* [Asking Questions](../asking-questions.md)

* [Command Reference](../command-reference.md)



