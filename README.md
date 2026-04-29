# clean-slate (`cs`)

Interactive CLI for spinning up isolated task workspaces under `~/projects/tasks/`. Each workspace bundles a task manifest, optional context documents, and git worktrees for one or more repos — so you can start a new bug investigation or feature branch without disturbing whatever you were last working on.

Designed to be driven interactively (fuzzy-search repo picker, huh prompts) **or** headlessly from scripts and AI agents via flags and JSON output.

## Install

### Homebrew (macOS, Linux)

```sh
brew install brian-lai/tap/clean-slate
```

> **Note:** macOS + Linux are supported. On Linux, `cs open` prints a `cd` command you can copy-paste — it can't launch a terminal the way iTerm2/Terminal.app do on macOS.

### From source

```sh
git clone https://github.com/brian-lai/clean-slate.git
cd clean-slate
make install                      # installs to ~/.local/bin/cs (or $GOBIN, $GOPATH/bin)
make install PREFIX=/usr/local    # override install location
```

`make install` resolves the install directory in this order: `$PREFIX/bin` → `$(go env GOBIN)` → `~/.local/bin` → `$(go env GOPATH)/bin`.

## Usage

```text
cs creates and manages isolated task workspaces under ~/projects/tasks/.

Available Commands:
  create       Create a new task workspace (interactive by default)
  list         List all task workspaces
  info         Show details for a task workspace
  status       Show git status for each worktree in a task
  open         Open a new terminal window in a task workspace
  add-context  Add supporting documents to an existing task's context/
  clean        Tear down a task workspace (remove worktrees and task directory)
  completion   Generate shell completion script

Flags:
  --json       Output in JSON format
  -v, --version
  -h, --help
```

### Quick start

```sh
cs create                              # interactive: prompts for name, JIRA, context, repos
cs create --name RNA-549 --description "fix login regression" --repos api,frontend
cs list --json
cs info RNA-549
cs open RNA-549                        # new terminal tab cd'd into the workspace (macOS)
cs clean RNA-549                       # remove worktrees + task dir
```

### Shell completion

```sh
cs completion zsh  > ~/.zsh/completions/_cs     # zsh
cs completion bash > ~/.bash_completion.d/cs    # bash
cs completion fish > ~/.config/fish/completions/cs.fish
```

Homebrew installs completions automatically.

## License

[MIT](LICENSE) — Copyright (c) 2026 Brian Lai
