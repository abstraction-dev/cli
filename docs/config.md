# Configuration

The `abstr config` command lets you inspect the CLI's current configuration without exposing sensitive values in your terminal. It is dispatched from the [Main](internal/cli/root.go), which forwards `config` arguments to the [runConfig](internal/cli/configcmd.go).

## Usage

```
abstr config [show|path] [--config <path>]
```

If no subcommand is given, `show` runs by default, as implemented by the [runConfig](internal/cli/configcmd.go).

## Subcommands

### `show` (default)

Prints a short summary of the active configuration, as implemented by the [runConfig](internal/cli/configcmd.go):

- **config** — the config file path currently in use, resolved by the config's [FilePath](internal/config/config.go)
- **workspace** — the active workspace name
- **api_key** — your API key, shown only in [redactKey](internal/cli/configcmd.go)
- **api_url** — the API URL actually being used, resolved by the [BaseURLResolved](internal/config/config.go), which prefers an environment override, then a configured value, then falls back to a default

### `path`

Prints only the config file location, again resolved by the [FilePath](internal/config/config.go), with nothing else — useful for scripting or quickly locating the file on disk.

## The `--config` flag

Pass `--config <path>` to point the command at an alternate config file instead of the default location. The value is passed straight through to the [Load](internal/config/config.go), which reads and decodes that file if it exists, or falls back to an empty configuration if it doesn't.

## Masked values

The API key is never printed in full. The [redactKey](internal/cli/configcmd.go) shows `(not set)` when no key is configured, `****` for very short keys, and otherwise only the first 8 characters followed by an ellipsis — enough to confirm which key is active without exposing the whole secret.

