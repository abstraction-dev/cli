# Getting Started

Welcome! This guide walks you through installing the CLI and taking your first few actions with it — no engineering background required.

## What is this tool?

The command-line tool you'll be using is called **abstr** — a compact program for chatting with the Abstraction service ("Astrid") right from your terminal. Under the hood it's a single compiled program built from the top-level entry point in [main](cmd/abstr/main.go), which simply hands off to a [Main](internal/cli/root.go) that decides what you're asking it to do.

## Installing and setting up

`abstr` is one small program (the CLI application package is literally described as being for "configuring access to the Abstraction service, selecting workspaces, and running ask/review workflows") — once you have the binary, there's nothing else to install. The very first time you run it with no arguments and no saved settings, it will offer to walk you through setup itself rather than making you edit files by hand — that's the job of the [bootstrapAPIKey](internal/cli/env.go), which opens your account's settings page in a browser and asks you to paste in an access key. If you just want to see everything the tool can do without running anything, running it with `-h` prints a full [printUsage](internal/cli/root.go) listing every command and flag.

Settings are kept in a small file the tool manages for you — by default in your home directory — resolved automatically by [Path](internal/config/config.go), so you never need to create or find this file yourself. You can also point it at a different file by passing `-config <path>`.

## Your first run: logging in

The easiest way to get going is simply to run:

```
abstr login
```

This is the guided setup path, driven by [runLogin](internal/cli/login.go): it loads (or creates) your settings, works out which server to talk to, walks you through getting an access key if you don't already have one, lets you pick which workspace you want to use, and then saves everything so you never have to repeat the process. For more detail on this flow, see [Logging In](logging-in.md).

## Picking a workspace

If your account has more than one workspace, `abstr` won't guess — [pickWorkspace](internal/cli/env.go) lists every workspace you have access to, marks your account's default, and asks you to choose a number. If you only have one workspace, it's selected for you automatically. You can revisit or change this choice at any time with `abstr workspace`, handled by [runWorkspace](internal/cli/workspace.go). Full details live in [Managing Workspaces](managing-workspaces.md).

## Asking your first question

Once you're logged in, just type your question after the command:

```
abstr how do I deploy to staging?
```

This is handled end-to-end by [runAsk](internal/cli/ask.go), which figures out what you're asking — a direct question, piped input, or nothing at all yet — through [resolveQuery](internal/cli/ask.go), makes sure your login and workspace are ready via [ensureConfigured](internal/cli/env.go), and then either answers immediately or opens an interactive session. You can also pipe a question in instead of typing it directly, e.g. `echo "question" | abstr`. See [Asking Questions](asking-questions.md) for everything this supports.

Here's how a first-time run flows from the moment you type `abstr` through setup and into your answer session.



```mermaid
flowchart TD
    N1["func Main(args []string) int"]
    N2["RETURN_NODE"]
    N3["func runAsk(ctx context.Context, args []string) int"]
    N4["RETURN_NODE"]
    N5["func ensureConfigured(ctx context.Context, opts runOptions, interactive bool) (*appEnv, error)"]
    N6["RETURN_NODE"]
    N7["func runREPL(env *appEnv, initialPR string) int"]
    N8["RETURN_NODE"]
    N9["Run the default ask flow"]
    N11["Arguments present?"]
    N13["switch args[0]"]
    N14["Version output"]
    N17["Help output"]
    N20["return runConfig(args[1:])"]
    N21["return runWorkspace(ctx, args[1:])"]
    N22["return runLogin(ctx, args[1:])"]
    N23["Initialize shared context"]
    N25["Dispatch execution mode"]
    N31["Initialize the CLI environment"]
    N36["Extract the query"]
    N41["Validate command-line input"]
    N49["Prepare the flag parser"]
    N51["return &appEnv{
    cfg: cfg,
    client: client,
    render: r,
    workspace: workspace,
}, nil"]
    N52["Resolve missing workspace interactively"]
    N64["Apply workspace override"]
    N67["workspace := cfg.WorkspaceResolved()"]
    N68["client := apiclient.New(baseURL, apiKey)"]
    N69["Ensure an API key exists"]
    N80["r := newRenderer()"]
    N81["Apply API URL override"]
    N85["Load configuration"]
    N89["return exitOK"]
    N90["Run the terminal session with failure handling"]
    N94["m := &replModel{
    env: env,
    sub: make(chan tea.Msg, 256),
    md: md,
    input: ti,
    sessionID: sessionID,
    activePR: initialPR,
}
m.entries = []transcriptEntry{transcriptEntry{
    entrySystem,
    'Chatting with Astrid. Type /help for commands, /exit to quit.',
}}"]
    N95["ti.Placeholder = 'Ask Astrid…  (/help, /exit)'
ti.ShowLineNumbers = false
ti.CharLimit = 8192
ti.SetHeight(inputHeight)
ti.Focus()"]
    N96["Keep the input area visually plain"]
    N98["ti.FocusedStyle.Prompt = userStyle
ti.BlurredStyle.Prompt = userStyle"]
    N99["Show the prompt only on the first line"]
    N101["sessionID, _ := uuidutil.New()
md := render.NewMDRenderer(render.HasDarkBackground(), 0)
ti := textarea.New()
ti.Prompt = inputPrompt"]
    N9 -->|RETURN_STEP| N2
    N14 -->|RETURN_STEP| N2
    N13 -->|case "version", "-v", "--version"| N14
    N17 -->|RETURN_STEP| N2
    N13 -->|case "help", "-h", "--help"| N17
    N20 -->|RETURN_STEP| N2
    N13 -->|case "config"| N20
    N21 -->|RETURN_STEP| N2
    N13 -->|case "workspace", "ws"| N21
    N22 -->|RETURN_STEP| N2
    N13 -->|case "login"| N22
    N11 -->|BRANCH_TRUE_CASE| N13
    N11 -->|BRANCH_FALSE_CASE| N9
    N23 -->|COMPUTE_STEP| N11
    N1 -->|COMPUTE_STEP| N23
    N25 -->|RETURN_STEP| N4
    N31 -->|RETURN_STEP| N4
    N31 -->|BRANCH_FALSE_CASE| N25
    N31 -->|FUNCTION_CALL| N5
    N6 -->|COMPUTE_STEP| N31
    N36 -->|RETURN_STEP| N4
    N36 -->|BRANCH_FALSE_CASE| N31
    N41 -->|RETURN_STEP| N4
    N41 -->|BRANCH_FALSE_CASE| N36
    N49 -->|COMPUTE_STEP| N41
    N3 -->|COMPUTE_STEP| N49
    N51 -->|RETURN_STEP| N6
    N52 -->|COMPUTE_STEP| N51
    N52 -->|BRANCH_FALSE_CASE| N51
    N52 -->|RETURN_STEP| N6
    N64 -->|COMPUTE_STEP| N52
    N64 -->|BRANCH_FALSE_CASE| N52
    N67 -->|COMPUTE_STEP| N64
    N68 -->|COMPUTE_STEP| N67
    N69 -->|COMPUTE_STEP| N68
    N69 -->|RETURN_STEP| N6
    N69 -->|BRANCH_FALSE_CASE| N68
    N80 -->|COMPUTE_STEP| N69
    N81 -->|COMPUTE_STEP| N80
    N81 -->|BRANCH_FALSE_CASE| N80
    N85 -->|RETURN_STEP| N6
    N85 -->|BRANCH_FALSE_CASE| N81
    N5 -->|COMPUTE_STEP| N85
    N89 -->|RETURN_STEP| N8
    N90 -->|RETURN_STEP| N8
    N90 -->|BRANCH_FALSE_CASE| N89
    N94 -->|COMPUTE_STEP| N90
    N95 -->|COMPUTE_STEP| N94
    N96 -->|COMPUTE_STEP| N95
    N98 -->|COMPUTE_STEP| N96
    N99 -->|COMPUTE_STEP| N98
    N101 -->|COMPUTE_STEP| N99
    N7 -->|COMPUTE_STEP| N101

```



## Going interactive

If you run `abstr` with no question typed (and you're at a normal terminal, not a script), it drops you into a live back-and-forth session instead of a one-shot answer — powered by [runREPL](internal/cli/repl.go), which sets up a scrollable conversation view and hands control to an interactive terminal program. Inside that session you can use slash commands like `/pr`, `/workspace`, `/new`, `/help`, and `/exit`. The full rundown is in [Interactive Mode (REPL)](interactive-mode.md).

## Checking your setup

Not sure what's configured right now? Run:

```
abstr config show
```

This is handled by [runConfig](internal/cli/configcmd.go), which prints your settings file location, active workspace, a safely redacted version of your API key, and the server address it will use — without ever printing your full key. Details on every setting are in [Configuration](configuration.md).

## Where to go next

That's everything you need to get moving — log in, pick a workspace, and start asking questions. For a complete list of every command and flag, see the [Command Reference](command-reference.md).

