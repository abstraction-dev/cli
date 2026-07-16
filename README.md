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
| `abstr config set auto_upgrade <true\|false>` | Toggle automatic upgrades. |
| `abstr upgrade` / `abstr upgrade --check` | Update to the latest release (or just check). |

### Staying up to date

`abstr upgrade` downloads the latest release, verifies it against the published
`sha256sums.txt`, and atomically replaces the running binary in place. Installs
owned by a package manager (Homebrew, Nix) are left untouched — upgrade those
with the manager instead. Self-upgrade is supported on macOS and Linux; on
Windows, download the latest release manually (or use WSL).

On a normal interactive run, `abstr` also checks for a newer release in the
background (at most once every 24h). By default it just prints a one-line notice;
enable `auto_upgrade` to have it apply updates automatically. The check is
skipped for local dev builds, under `$CI`, when output isn't a terminal, or when
`ABSTR_NO_UPDATE_CHECK` is set.

### Configuration & precedence

`abstr` reads `~/.abstr.json` (override with `--config` / `$ABSTR_CONFIG`).
Effective values resolve as **flag > env > file > default**:

| Setting | Flag | Env | File key |
|---|---|---|---|
| Workspace | `-w`/`--workspace` | `ABSTR_WORKSPACE` | `workspace` |
| API key | — | `ABSTR_API_KEY` | `api_key` |
| Base URL | `--api-url` | `ABSTR_API_URL` | `api_base_url` |

Output is pipe-friendly: only the answer is written to stdout; prompts, status,
and errors go to stderr. Color auto-disables when stdout is not a terminal.
