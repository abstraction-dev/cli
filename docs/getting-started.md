# Getting Started

## Getting Started

Welcome! This tool — called **abstr** on the command line — lets you chat with the Astrid assistant right from your terminal: ask questions, get instant answers, and manage which "workspace" (project/account) you're working in.

### 1. Installing the tool

Once installed, you run everything through a single command-line program. The program's very first job, when you launch it, is to hand off your typed command to a central dispatcher, using the [main](cmd/abstr/main.go) which passes along whatever you typed and exits with the right status when it's done.

### 2. Logging in (one-time setup)

Before you can ask questions, you need to connect the tool to your account. Typing `abstr login` kicks off the [runLogin](internal/cli/login.go), which walks you through the whole process automatically:

- It loads any settings you already have saved, and figures out which server to talk to.
- If you don't have an API key yet, it opens your web browser to the account settings page for you and asks you to paste in a key, using the [bootstrapAPIKey](internal/cli/env.go) — it even gives you three attempts if you mistype or paste the wrong thing.
- Once your key is confirmed, it asks which workspace you'd like to use — if your account only has one, it's picked automatically, and if you have several, the [pickWorkspace](internal/cli/env.go) shows you a numbered list to choose from.
- Finally, it saves everything to a local settings file so you don't have to log in again next time.

Here's what that whole first-time setup looks like under the hood, from typing the command to being logged in.



```mermaid
flowchart TD
    N1["func Main(args []string) int"]
    N2["RETURN_NODE"]
    N3["func runLogin(ctx context.Context, args []string) int"]
    N4["RETURN_NODE"]
    N5["func runAsk(ctx context.Context, args []string) int"]
    N6["RETURN_NODE"]
    N7["Run the default ask flow"]
    N9["Arguments present?"]
    N11["switch args[0]"]
    N12["Version output"]
    N15["Help output"]
    N18["return runConfig(args[1:])"]
    N19["return runWorkspace(ctx, args[1:])"]
    N20["return runLogin(ctx, args[1:])"]
    N21["Initialize shared context"]
    N23["return exitOK"]
    N24["r.Info('Logged in. Config saved to ' + cfg.FilePath())"]
    N25["Handle save error"]
    N27["return exitRuntime"]
    N28["r.Error('save config: ' + err.Error())"]
    N29["cfg.Workspace = ws"]
    N30["Handle workspace selection error"]
    N32["return exitRuntime"]
    N33["r.Error(err.Error())"]
    N34["cfg.APIKey = key
ws, err := pickWorkspace(ctx, apiclient.New(baseURL, key), r)"]
    N35["Handle authentication error"]
    N37["return exitAuth"]
    N38["r.Error(err.Error())"]
    N39["r := newRenderer()
key, err := bootstrapAPIKey(ctx, r, baseURL)"]
    N40["Apply API URL override"]
    N42["baseURL = apiURL
cfg.APIBaseURL = apiURL"]
    N43["baseURL := cfg.BaseURLResolved()"]
    N44["Handle config load error"]
    N46["return exitRuntime"]
    N47["fmt.Fprintln(os.Stderr, 'abstr: ' + err.Error())"]
    N48["cfg, err := config.Load(configPath)"]
    N49["Validate command-line flags"]
    N51["return exitUsage"]
    N52["fs := flag.NewFlagSet('abstr login', flag.ContinueOnError)
fs.SetOutput(os.Stderr)
var configPath string
var apiURL string
fs.StringVar(&configPath, 'config', '', 'config file path')
fs.StringVar(&apiURL, 'api-url', '', 'API base URL')"]
    N53["Dispatch execution mode"]
    N59["Initialize the CLI environment"]
    N64["Extract the query"]
    N69["Validate command-line input"]
    N77["Prepare the flag parser"]
    N7 -->|RETURN_STEP| N2
    N12 -->|RETURN_STEP| N2
    N11 -->|case "version", "-v", "--version"| N12
    N15 -->|RETURN_STEP| N2
    N11 -->|case "help", "-h", "--help"| N15
    N18 -->|RETURN_STEP| N2
    N11 -->|case "config"| N18
    N19 -->|RETURN_STEP| N2
    N11 -->|case "workspace", "ws"| N19
    N20 -->|RETURN_STEP| N2
    N11 -->|case "login"| N20
    N9 -->|BRANCH_TRUE_CASE| N11
    N9 -->|BRANCH_FALSE_CASE| N7
    N21 -->|COMPUTE_STEP| N9
    N1 -->|COMPUTE_STEP| N21
    N23 -->|RETURN_STEP| N4
    N24 -->|COMPUTE_STEP| N23
    N27 -->|RETURN_STEP| N4
    N28 -->|COMPUTE_STEP| N27
    N25 -->|BRANCH_TRUE_CASE| N28
    N25 -->|BRANCH_FALSE_CASE| N24
    N29 -->|COMPUTE_STEP| N25
    N32 -->|RETURN_STEP| N4
    N33 -->|COMPUTE_STEP| N32
    N30 -->|BRANCH_TRUE_CASE| N33
    N30 -->|BRANCH_FALSE_CASE| N29
    N34 -->|COMPUTE_STEP| N30
    N37 -->|RETURN_STEP| N4
    N38 -->|COMPUTE_STEP| N37
    N35 -->|BRANCH_TRUE_CASE| N38
    N35 -->|BRANCH_FALSE_CASE| N34
    N39 -->|COMPUTE_STEP| N35
    N42 -->|COMPUTE_STEP| N39
    N40 -->|BRANCH_TRUE_CASE| N42
    N40 -->|BRANCH_FALSE_CASE| N39
    N43 -->|COMPUTE_STEP| N40
    N46 -->|RETURN_STEP| N4
    N47 -->|COMPUTE_STEP| N46
    N44 -->|BRANCH_TRUE_CASE| N47
    N44 -->|BRANCH_FALSE_CASE| N43
    N48 -->|COMPUTE_STEP| N44
    N51 -->|RETURN_STEP| N4
    N49 -->|BRANCH_TRUE_CASE| N51
    N49 -->|BRANCH_FALSE_CASE| N48
    N52 -->|COMPUTE_STEP| N49
    N3 -->|COMPUTE_STEP| N52
    N53 -->|RETURN_STEP| N6
    N59 -->|RETURN_STEP| N6
    N59 -->|BRANCH_FALSE_CASE| N53
    N64 -->|RETURN_STEP| N6
    N64 -->|BRANCH_FALSE_CASE| N59
    N69 -->|RETURN_STEP| N6
    N69 -->|BRANCH_FALSE_CASE| N64
    N77 -->|COMPUTE_STEP| N69
    N5 -->|COMPUTE_STEP| N77

```



### 3. Checking your setup anytime

If you're ever unsure what's configured, `abstr config` shows you a quick summary — your settings file location, your active workspace, and a safely-hidden version of your API key — via the [runConfig](internal/cli/configcmd.go). If you just want to see which file your settings live in, `abstr config path` does that directly.

### 4. Asking your first question

You've got two easy ways to ask something, both handled by the [runAsk](internal/cli/ask.go):

- **Ask directly**: type your question right after the command, e.g. `abstr "What does this pull request change?"`.
- **Pipe it in**: send text from another tool straight into abstr, and it will read that as your question.

Behind the scenes, the tool figures out where your question is coming from — typed directly, piped in, or "none yet" — using the [resolveQuery](internal/cli/ask.go), and makes sure your setup (workspace, key, connection) is ready via the [ensureConfigured](internal/cli/env.go) before sending anything. For a single question, the [runImmediate](internal/cli/ask.go) prints a nicely formatted, styled reply if you're in a normal terminal window, or plain clean text if you're piping the output somewhere else.

### 5. Chatting interactively

If you just run `abstr` with no question at all in a normal terminal window, it opens a full interactive chat session instead — a scrollable, styled conversation window — powered by the [runREPL](internal/cli/repl.go), which sets up the chat display, remembers your conversation, and keeps the session running until you exit.

### 6. Switching or checking workspaces

If you belong to more than one workspace, `abstr workspace` (or the shorter `abstr ws`) lets you list them all or switch the active one, through the [runWorkspace](internal/cli/workspace.go) — for example `abstr workspace list` shows every workspace with your current one marked, and `abstr workspace use <name>` switches to a different one by name.

### 7. Getting help anytime

Typing `abstr help` (or `-h` / `--help`) prints a full summary of everything the tool can do — asking questions, piping input, logging in, managing workspaces, and checking configuration — straight from the built-in [printUsage](internal/cli/root.go). Typing `abstr version` (or `-v` / `--version`) simply prints which version you're running, and both of these, along with all the commands above, are routed by the same central [Main](internal/cli/root.go) that reads your very first word after `abstr` and sends you to the right place.

