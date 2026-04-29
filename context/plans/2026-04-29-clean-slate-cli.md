# Clean Slate CLI (`cs`)

> **Master plan.** Phase-specific details are in sub-plan files. Load only the phase you're working on.

---

## Objective

Build `cs`, a Go CLI tool for creating and managing isolated task workspaces under `~/projects/tasks/`. The tool serves both human-interactive and headless (agent) use cases — interactive TUI prompts when run by a person, flags + JSON output when driven by an agent. It is workflow-agnostic: it creates the workspace structure, the user/agent decides how to use it.

---

## Core Principles

1. **Interactive-first, headless-capable.** Every input has both a TUI prompt and a CLI flag. Missing flags trigger interactive prompts; all flags provided skips prompts entirely.
2. **Workspace-only, workflow-agnostic.** The tool creates directories, worktrees, and manifests. It has no knowledge of PARA, branching strategies, or CI workflows.
3. **Manifest as source of truth.** Every task directory contains a `task.json` that fully describes the workspace. All commands read from this manifest — no hidden state.
4. **Idempotent where possible.** Re-running `cs create` for an existing task should fail with a clear message, not corrupt state. `cs clean` should handle partially-created workspaces gracefully.
5. **Agent-friendly output.** `--json` flag on every command produces machine-parseable JSON to stdout. Human-readable output is the default.

---

## Architecture

```
cs (binary)
├── cmd/                    # Cobra command definitions
│   ├── root.go             # Root command, global flags (--json)
│   ├── create.go           # cs create <name>
│   ├── list.go             # cs list
│   ├── info.go             # cs info <name>
│   ├── open.go             # cs open <name>
│   ├── status.go           # cs status <name>
│   ├── clean.go            # cs clean <name>
│   └── add_context.go      # cs add-context <name> <path>...
├── internal/
│   ├── config/             # Configuration (base paths, defaults)
│   ├── manifest/           # task.json read/write/schema
│   ├── workspace/          # Directory creation, file copying
│   ├── git/                # Git worktree operations
│   ├── tui/                # Interactive prompts (fuzzy select, text input)
│   └── terminal/           # Terminal launcher (iTerm2/Terminal.app)
├── main.go                 # Entry point
├── go.mod
└── go.sum
```

### Data Flow: `cs create`

```
1. Parse flags → resolve inputs (fill from flags or prompt interactively)
2. Validate: task name not taken, repos exist in ~/projects/repos/
3. Create ~/projects/tasks/<name>/
4. Create ~/projects/tasks/<name>/context/ (copy any provided docs)
5. For each selected repo:
   a. Detect default branch (git symbolic-ref refs/remotes/origin/HEAD)
   b. git worktree add ~/projects/tasks/<name>/<repo>/ -b ws/<name> <default-branch>
6. Write ~/projects/tasks/<name>/task.json
7. Output result (human-readable or JSON)
```

---

## Architecture Decisions

| Decision | Choice | Rationale | Alternatives Rejected |
|----------|--------|-----------|----------------------|
| CLI framework | Cobra | De facto Go standard, subcommand support, flag parsing, shell completions built-in | urfave/cli (less ecosystem), raw flag (no subcommands) |
| Interactive TUI | charmbracelet/huh (forms) + bubbletea (fuzzy select) | Charm ecosystem is actively maintained, composable, handles complex interactions like fuzzy multi-select | promptui (abandoned), survey (abandoned Go port), AlecAivazis/survey (archived) |
| Fuzzy matching | sahilm/fuzzy | Lightweight, well-tested fuzzy matcher. Integrates cleanly with bubbletea list model | Implementing from scratch (unnecessary), fzf subprocess (external dependency) |
| JSON output | encoding/json stdlib | No external dependency needed, sufficient for flat manifest structure | protobuf (overkill), jsoniter (premature optimization) |
| Manifest format | JSON (`task.json`) | Human-readable, universally parseable, no schema library needed | YAML (extra dependency), TOML (less familiar to agents), SQLite (overkill) |
| Worktree branch naming | `ws/<task-name>` | Simple convention, clearly identifies workspace branches, avoids collision with user branches (feature/, fix/, etc.) | `task/<name>` (fine but ws/ is shorter), bare name (collision risk) |
| Terminal launcher | AppleScript via `osascript` | Works with both Terminal.app and iTerm2, no external dependency | tmux (not everyone uses it), open command (can't control cd) |

---

## Responsibility Split

| Responsibility | Owner |
|---------------|-------|
| CLI flag parsing, subcommand routing | `cmd/` package (Cobra) |
| Interactive user input (prompts, fuzzy select) | `internal/tui/` |
| Task directory creation, context file copying | `internal/workspace/` |
| Git worktree add/remove/list, default branch detection | `internal/git/` |
| task.json schema, read, write, validate | `internal/manifest/` |
| Base paths, configurable defaults | `internal/config/` |
| Terminal window launching | `internal/terminal/` |

---

## Graceful Degradation

| Failure Scenario | Expected Behavior |
|-----------------|-------------------|
| Repo path in `~/projects/repos/<name>` is not a git repo | Skip with warning, continue with other repos. Report skipped repos in output. |
| `git worktree add` fails (branch already exists, locked) | Report error for that repo, continue with others. Partial success is valid — manifest records which repos succeeded. |
| Default branch detection fails (no remote HEAD) | Fall back to `main`, then `master`, then fail with actionable error message. |
| Task directory already exists | Fail fast with error: "task '<name>' already exists at <path>". No partial overwrite. |
| iTerm2/Terminal.app not available for `open` | Fall back to printing the path with a `cd` suggestion. |
| Context doc path doesn't exist | Warn and skip that path, continue with others. |
| No repos in `~/projects/repos/` | Show empty selection, allow creating a task with zero repos. |
| Worktree partially created during `clean` | Remove what exists, skip what doesn't, always remove manifest last. |

---

## Phase Overview

| Phase | Title | Scope |
|-------|-------|-------|
| **1** | Foundation — project scaffold, manifest, and `create` (headless) | Go module, config, manifest schema, workspace creation, git worktree integration, `create` command with flags only (no TUI). All core logic testable. |
| **2** | Interactive TUI + remaining commands | Charm-based interactive prompts for `create`, fuzzy repo selector, `list`, `info`, `status`, `open`, `clean`, `add-context` commands. |
| **3** | Polish + install | Shell completions, error messages, `--json` flag consistency audit, Makefile, install script, README. |

### Progressive Regression Rule

```
Phase 1 → Unit tests for config, manifest, workspace, git packages. Integration test for headless create flow.
Phase 2 → + Command-level tests for all subcommands. TUI component tests (model state, not visual).
Phase 3 → + E2E acceptance tests (binary invocation). Shell completion tests.
```

---

## Execution Plan

1. **Review all phases** — Ensure entire approach is sound before starting
2. **Execute Phase 1** — Run `/para:execute --phase=1`
3. **Summarize Phase 1** — Run `/para:summarize --phase=1`
4. **Review & Merge Phase 1**
5. **Execute Phase 2** — Run `/para:execute --phase=2`
6. **Continue** — Repeat for remaining phases
7. **Final Verification** — Ensure overall success criteria met
8. **Archive** — Run `/para:archive`

### Worktree & Branch Strategy

| Phase | Branch | Worktree Path |
|-------|--------|---------------|
| Phase 1 | `para/clean-slate-cli-phase-1` | `.para-worktrees/clean-slate-cli-phase-1` |
| Phase 2 | `para/clean-slate-cli-phase-2` | `.para-worktrees/clean-slate-cli-phase-2` |
| Phase 3 | `para/clean-slate-cli-phase-3` | `.para-worktrees/clean-slate-cli-phase-3` |

---

## New Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `cmd/` | `cmd/*.go` | Cobra command definitions for all subcommands |
| `internal/config` | `internal/config/config.go` | Centralized path configuration (tasks dir, repos dir) |
| `internal/manifest` | `internal/manifest/manifest.go` | task.json struct, read/write, validation |
| `internal/workspace` | `internal/workspace/workspace.go` | Directory creation, context file copying |
| `internal/git` | `internal/git/worktree.go` | Git worktree add/remove/list, default branch detection |
| `internal/tui` | `internal/tui/*.go` | Charm-based interactive prompts and fuzzy selector |
| `internal/terminal` | `internal/terminal/launch.go` | iTerm2/Terminal.app window launcher via osascript |

---

## Manifest Schema (`task.json`)

`context_docs` stores paths **relative to the task directory** for portability. Callers resolve against the task dir at read time.

Task names must match `[a-zA-Z0-9._-]+` (validated by `manifest.Validate`).

```json
{
  "name": "my-task",
  "created_at": "2026-04-29T11:30:00Z",
  "jira_ticket": "PROJ-549",
  "description": "Investigate dataset sync failures",
  "repos": [
    {
      "name": "alpha",
      "source": "/Users/blai/projects/repos/alpha",
      "worktree_path": "/Users/blai/projects/tasks/my-task/alpha",
      "worktree_branch": "ws/my-task",
      "base_branch": "main"
    }
  ],
  "context_docs": [
    "context/error-log.txt"
  ]
}
```

---

## Sub-Plans

- `2026-04-29-clean-slate-cli-phase-1.md` — Foundation: Go module, config, manifest, workspace, git worktree, headless `create` command
- `2026-04-29-clean-slate-cli-phase-2.md` — Interactive TUI + all remaining commands (list, info, status, open, clean, add-context)
- `2026-04-29-clean-slate-cli-phase-3.md` — Polish: shell completions, `--json` audit, Makefile, install script
