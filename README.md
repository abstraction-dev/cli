# abstr

The Abstraction CLI — chat with [Astrid](https://abstraction.dev) from your terminal.

`abstr` is a thin interface to Abstraction's agent chat. Every conversation is
ephemeral; nothing is stored server-side.

## Install

macOS / Linux:

```sh
curl -fsSL https://abstraction.dev/install.sh | sh
```

This downloads the latest prebuilt binary for your OS/arch from
[GitHub Releases](https://github.com/abstraction-dev/cli/releases/latest) and
installs it to `/usr/local/bin` (or `~/.local/bin` if that isn't writable).

Windows: download `abstr_windows_amd64.zip` from the
[latest release](https://github.com/abstraction-dev/cli/releases/latest), extract it,
and put `abstr.exe` somewhere on your `PATH`.

## First run

```sh
abstr login
```

Opens app.abstraction.dev so you can create an API key, prompts you to paste it,
then lets you pick a workspace. Both are stored in `~/.abstr.json` (mode `0600`).

## Usage

```sh
abstr "how does authentication work?"        # ask a question (buffered output)
echo "trace the login flow" | abstr          # ask via a pipe
abstr                                         # interactive session (streamed)
abstr -w <workspace-slug> "explain workers"   # one-off workspace override
abstr --pr 4821 "does this PR break auth?"    # scope to a PR (number)
abstr --pr https://github.com/o/r/pull/4821 "…"   # …or a PR URL
```

Interactive commands: `/pr <url|number>`, `/pr clear`, `/workspace [slug]`,
`/new`, `/help`, `/exit`.

### Subcommands

| Command | Description |
|---|---|
| `abstr login` | Store your API key and pick a workspace. |
| `abstr workspace list` | List your workspaces (`*` marks the current one). |
| `abstr workspace use <slug\|name>` | Switch the current workspace. |
| `abstr workspace` | Interactive workspace picker. |
| `abstr config path` / `abstr config show` | Inspect configuration. |
| `abstr config set <key> <value>` | Set `browser_command` or `history_limit`. |
| `abstr history` / `abstr history -n 50` | List locally stored exchanges. |
| `abstr history path` / `abstr history clear` | Locate or delete the history file. |

### Local history

Buffered asks are appended to `~/.abstr-history.json` (override with
`$ABSTR_HISTORY`), newest entries kept up to `history_limit` (default 50). Set
`history_limit` to a negative value to turn recording off. The file holds the
question and the answer text only, is written with `0600` perms, and is never
sent anywhere — conversations themselves stay server-side and ephemeral.

Updating is handled by your installer: re-run the install script, or use the
package manager that owns the binary.

### Configuration & precedence

`abstr` reads `~/.abstr.json` (override with `--config` / `$ABSTR_CONFIG`).
Effective values resolve as **flag > env > file > default**:

| Setting | Flag | Env | File key |
|---|---|---|---|
| Workspace | `-w`/`--workspace` | `ABSTR_WORKSPACE` | `workspace` |
| API key | — | `ABSTR_API_KEY` | `api_key` |
| Base URL | `--api-url` | `ABSTR_API_URL` | `api_base_url` |
| Browser command | — | `ABSTR_BROWSER` | `browser_command` |
| History limit | — | — | `history_limit` |
| History file | — | `ABSTR_HISTORY` | — |

Output is pipe-friendly: only the answer is written to stdout; prompts, status,
and errors go to stderr. Color auto-disables when stdout is not a terminal.
