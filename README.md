# abstr

The Abstraction CLI — chat with [Astrid](https://abstraction.dev) from your terminal.

`abstr` is a thin interface to Abstraction's agent chat. Every conversation is
ephemeral; nothing is stored server-side.

## Install

```sh
go install github.com/abstraction-dev/cli/cmd/abstr@latest
```

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
