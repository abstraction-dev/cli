# Managing Workspaces

I have enough grounding to write the guide.# Managing Workspaces

Workspaces let you switch between different environments in the CLI. All workspace actions are handled by the [runWorkspace](internal/cli/workspace.go), which loads your configuration and API credentials before running any subcommand.



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



## Listing workspaces

See every workspace available to your account with the `list` action, which [Workspaces](internal/apiclient/client.go) and marks your active one with `*`:

```
abstr workspace list
```

## Picking one

Run the command with no arguments to let the CLI [pickWorkspace](internal/cli/env.go) — it auto-selects if you only have one, or prompts you to choose when there are several:

```
abstr workspace
```

## Switching workspace

Use the `use` action to [switchWorkspaceCmd](internal/cli/repl.go) by slug or name, then [saveWorkspace](internal/cli/workspace.go) as your active workspace:

```
abstr workspace use <slug|name>
```

You must be logged in first — see [Logging In](login.md). For all available flags and actions, check the [Command Reference](reference.md).

