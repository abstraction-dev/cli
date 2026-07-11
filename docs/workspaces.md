# Managing Workspaces

---

Workspaces are the boundary the CLI uses to keep one account's questions, pull requests, and configuration separate from another's. Every command that talks to the backend — asking a question, listing pull requests, and so on — runs "inside" whichever workspace is currently active, and the tool always gives you a way to see, change, or pick that active workspace before it lets you continue.

## What counts as a workspace

A workspace is just a small record: a short slug used to identify it, a human-readable display name, and a flag marking whether it is the account's default. That's the entire [Workspace](internal/apiclient/types.go) the CLI keeps track of — nothing more.

The active workspace itself lives in your saved configuration. The CLI reads it back through [WorkspaceResolved](internal/config/config.go), which checks for an environment override first and only falls back to the value saved on disk if no override is set. For the full picture of how that saved configuration file works, see [Configuration](configuration.md).

## The `workspace` command

Everything workspace-related on the command line funnels through [runWorkspace](internal/cli/workspace.go). Before it does anything else it loads your configuration, resolves your API key, and — if you're not logged in — stops and tells you to run `abstr login` first (see [Logging In](logging-in.md)). Once it has a valid API client, it looks at what you typed after `workspace` and does one of three things:

- **No extra word (`abstr workspace`)** — it picks a workspace for you using [pickWorkspace](internal/cli/env.go). If your account only has one workspace, it is selected automatically; if there are several, you're shown a numbered list (with the account's default marked) and asked to type a number until you enter a valid one.
- **`abstr workspace list`** — it fetches every workspace your account can see and prints each one's slug and name, marking whichever one is currently active with a `*`.
- **`abstr workspace use <slug or name>`** — it looks up the workspace you named (matching by exact slug or by name, ignoring case) and switches to it. If nothing matches, it tells you plainly that no workspace matched what you typed.

Whichever way a workspace gets chosen, the result is handed to [saveWorkspace](internal/cli/workspace.go), which updates the saved workspace and writes it to disk — and if writing to disk fails for any reason, it puts things back exactly as they were rather than leaving the app thinking the switch succeeded.

```mermaid
flowchart TD
    N1["func runWorkspace(ctx context.Context, args []string) int"]
    N2["RETURN_NODE"]
    N3["switch action"]
    N4["Handle unknown workspace commands"]
    N7["Switch to a named workspace"]
    N20["List available workspaces"]
    N32["Pick and save a workspace"]
    N38["if len(sub) > 0"]
    N39["action = sub[0]"]
    N40["action := 'pick'"]
    N41["client := apiclient.New(baseURL, key)"]
    N42["Require authentication"]
    N47["r := newRenderer()"]
    N48["Apply API URL override"]
    N52["if err != nil"]
    N53["return exitRuntime"]
    N54["fmt.Fprintln(os.Stderr, 'abstr: ' + err.Error())"]
    N55["sub := fs.Args()
cfg, err := config.Load(configPath)"]
    N56["Validate command-line parsing"]
    N59["Initialize workspace command flags"]
    N4 -->|RETURN_STEP| N2
    N3 -->|case | N4
    N7 -->|RETURN_STEP| N2
    N3 -->|case "use"| N7
    N20 -->|RETURN_STEP| N2
    N3 -->|case "list"| N20
    N32 -->|RETURN_STEP| N2
    N3 -->|case "pick"| N32
    N39 -->|COMPUTE_STEP| N3
    N38 -->|BRANCH_TRUE_CASE| N39
    N38 -->|BRANCH_FALSE_CASE| N3
    N40 -->|COMPUTE_STEP| N38
    N41 -->|COMPUTE_STEP| N40
    N42 -->|RETURN_STEP| N2
    N42 -->|BRANCH_FALSE_CASE| N41
    N47 -->|COMPUTE_STEP| N42
    N48 -->|COMPUTE_STEP| N47
    N48 -->|BRANCH_FALSE_CASE| N47
    N53 -->|RETURN_STEP| N2
    N54 -->|COMPUTE_STEP| N53
    N52 -->|BRANCH_TRUE_CASE| N54
    N52 -->|BRANCH_FALSE_CASE| N48
    N55 -->|COMPUTE_STEP| N52
    N56 -->|RETURN_STEP| N2
    N56 -->|BRANCH_FALSE_CASE| N55
    N59 -->|COMPUTE_STEP| N56
    N1 -->|COMPUTE_STEP| N59

```



## Switching workspaces from inside a conversation

You don't have to leave an interactive session to change workspaces. Typing `/workspace` or `/ws` inside the REPL is handled by [runCommand](internal/cli/repl.go): typing it with no name opens the same picker experience described above, and typing it with a name or slug — e.g. `/workspace My Team` — resolves it through [switchWorkspaceCmd](internal/cli/repl.go), matching either the exact slug or a case-insensitive name match.

Once a match is found, [applySwitch](internal/cli/repl.go) does the actual work: it saves the new workspace choice, updates the running session's active workspace and display name, clears out any pull request you had selected, and starts a brand-new conversation — so nothing from the old workspace bleeds into the new one. If saving fails, it rolls the change back and tells you why in the transcript rather than pretending the switch went through. For everything else you can do inside a live session, see [Interactive Mode (REPL)](interactive-mode.md).

## Where this fits in

Workspace selection sits upstream of almost everything else the CLI does — asking questions (see [Asking Questions](asking-questions.md)) and other backend calls all run against whatever workspace is currently active, whether that was set from the command line or from inside the REPL. If you're new to the tool altogether, [Getting Started](getting-started.md) walks through the first-run setup, and the full list of flags and subcommands — workspace included — is catalogued in the [Command Reference](command-reference.md).

