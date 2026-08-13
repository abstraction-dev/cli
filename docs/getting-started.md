# Getting Started

# Getting Started with abstr

abstr is a command-line tool that lets you chat with Astrid, an AI assistant, right from your terminal. Instead of switching to a browser, you can ask questions about your work, get answers about a specific pull request, or just have a quick back-and-forth conversation — all without leaving the command line.

## Installation

Installing abstr takes one command. Open your terminal and run:

```
curl -fsSL https://abstraction.dev/install.sh | sh
```

This downloads and runs the official install script, which automatically fetches and installs the latest released version of abstr for your system. There's nothing to configure beforehand — once the command finishes, the `abstr` command is ready to use.

To check that the install worked, run:

```
abstr --version
```

You should see the installed version number printed back to you.

## What abstr Does

At its core, abstr gives you a direct line to Astrid from your terminal. You can:

* Ask a question and get an answer immediately, as a single command.

* Open an interactive chat session and have an ongoing conversation.

* Scope your question to a specific GitHub pull request, so Astrid answers with that context in mind.

* Work across multiple workspaces (for example, if you belong to more than one team or account) and switch between them whenever you need to.

## Logging In

Before you can ask questions, you need to connect abstr to your account. Run:

```
abstr login
```

This starts a short setup flow:

1. abstr will prompt you to provide an API key. If you don't have one yet, it will guide you through creating one.
2. Once you provide the key, abstr checks that it's valid.
3. You'll then be asked to choose a workspace to use — if your account only has one workspace, abstr selects it automatically; otherwise you'll see a numbered list to pick from.
4. abstr saves your API key and chosen workspace to a small configuration file on your computer (by default `~/.abstr.json`), so you won't need to log in again on future runs.

When everything succeeds, you'll see a confirmation message letting you know you're logged in and where your settings were saved.

## Choosing or Switching a Workspace

If you belong to more than one workspace, or you simply want to switch which one abstr is pointed at, use the `workspace` command.

**See your available workspaces:**

```
abstr workspace list
```

This prints every workspace you have access to, with an asterisk (`*`) marking the one currently in use.

**Switch to a specific workspace:**

```
abstr workspace use <workspace-name-or-slug>
```

You can type either the workspace's name or its short identifier (slug). If the name has multiple words, you don't need to add quotes — just type it as-is.

**Pick a workspace interactively:**

```
abstr workspace
```

Running the command with no extra options opens the same picker you saw during login, letting you choose from a list.

Your chosen workspace is remembered automatically, so you only need to switch it when you actually want to change context.

## Asking a Question

There are two ways to ask Astrid something: as a quick one-off command, or as an ongoing conversation.

### Ask a single question directly

Simply type your question after `abstr`:

```
abstr "What changed in the last deployment?"
```

abstr sends your question, streams back the answer in your terminal, and then returns you to your regular command prompt. This is the fastest way to get a single answer without starting a whole session.

You can also pipe a question in from another command:

```
echo "Summarize the open pull requests" | abstr
```

### Start an interactive session

If you run `abstr` with no question at all, it opens an interactive chat session instead of asking for a single answer:

```
abstr
```

In this mode you can have a back-and-forth conversation, asking follow-up questions the way you would in a chat app. While inside the session, a few handy commands are available:

* `/pr <url>` — focus the conversation on a specific GitHub pull request

* `/pr clear` — remove that pull request focus

* `/workspace [slug]` — switch workspaces without leaving the session

* `/new` — start a fresh conversation

* `/help` — show a reminder of available commands

* `/exit` — leave the interactive session

### Asking about a specific pull request

If you want an answer scoped to a particular pull request, add the `-pr` flag with the pull request's GitHub URL:

```
abstr -pr https://github.com/your-org/your-repo/pull/123 "What does this change do?"
```

### Choosing a workspace just for one question

If you want to ask a question against a workspace other than your current default, use the `-w` (or `-workspace`) flag:

```
abstr -w your-workspace-slug "Any open issues I should know about?"
```

This only affects that single command — your saved default workspace stays the same.

## Getting Help

At any time, you can see a summary of available commands and flags by running:

```
abstr help
```

or

```
abstr -h
```



