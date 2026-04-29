# Phase 2: Interactive TUI + Remaining Commands

> **Parent plan:** `2026-04-29-clean-slate-cli.md`
> **Prerequisite:** Phase 1 merged (core packages and headless `create` working)
> **Outcome:** `cs create` works interactively with fuzzy repo selection. All subcommands (`list`, `info`, `status`, `open`, `clean`, `add-context`) are functional in both interactive and headless modes.

---

## Objective

Layer interactive TUI prompts on top of the headless `create` command using the Charm ecosystem (bubbletea + huh). Implement all remaining subcommands. Every command supports both `--json` output and human-readable output.

---

## Key Context from Master Plan

**Relevant principles:**
- Interactive-first, headless-capable — missing flags trigger interactive prompts
- Workspace-only, workflow-agnostic — no PARA or workflow knowledge
- Manifest as source of truth — all commands read from `task.json`
- Agent-friendly output — `--json` on every command

**Relevant architecture decisions:**
- charmbracelet/huh for form prompts, bubbletea for fuzzy multi-select
- sahilm/fuzzy for fuzzy matching in the repo selector
- AppleScript via `osascript` for terminal launching (`open` command)

**Contracts this phase implements:**

```go
// tui.Prompter — interactive input collection
type CreateInputs struct {
    Name        string
    JiraTicket  string
    Description string
    ContextDocs []string
    Repos       []string
}

func PromptCreate(repoNames []string, prefilled CreateInputs) (CreateInputs, error)
// Prompts only for fields not already set in prefilled.
// repoNames is the full list for fuzzy selection.

// terminal.Launcher
func LaunchTerminal(dir string) error
// Opens a new terminal window cd'd into dir.
// Detects iTerm2 vs Terminal.app.
```

---

## Scope

### In Scope

- `internal/tui/` — interactive prompts for `create` (text inputs, fuzzy repo multi-select)
- Wire TUI into `cmd/create.go` — prompt for missing flags
- `cmd/list.go` — list all tasks (name, date, repo count, description snippet)
- `cmd/info.go` — display full task manifest
- `cmd/status.go` — git status across all worktrees (`--verbose` for rich output)
- `cmd/open.go` — launch terminal in task directory
- `cmd/clean.go` — remove worktrees, optionally archive task directory
- `cmd/add_context.go` — copy additional docs into existing task's context/
- `internal/terminal/` — iTerm2/Terminal.app launcher
- Tests for TUI model state, all command logic, terminal launcher

### Out of Scope

- Shell completions — Phase 3
- Makefile, install script, README — Phase 3
- E2E acceptance tests (binary invocation) — Phase 3

---

## Implementation Steps

> Each checklist item below maps to one git commit. The checkbox text is the commit message.
> Tests come BEFORE the implementation they cover (TDD).

- [ ] **Add Charm dependencies (bubbletea, huh, lipgloss, bubbles, sahilm/fuzzy)**
  - `go get github.com/charmbracelet/bubbletea github.com/charmbracelet/huh github.com/charmbracelet/lipgloss github.com/charmbracelet/bubbles github.com/sahilm/fuzzy`
  - **File(s):** `go.mod`, `go.sum`
  - **Tests:** None (dependency only — verify with `go build`)

- [ ] **Write tests for TUI prompt logic**
  - Test that prefilled fields skip prompting (unit test on input resolution, not visual)
  - Test that fuzzy matching filters repo names correctly
  - Test that all-flags-provided returns inputs without prompting
  - **File(s):** `internal/tui/prompt_test.go`
  - **Tests:** `TestPrefilledSkipsPrompt` — all fields set in prefilled, assert no prompt needed flag. `TestFuzzyMatchRepos` — fuzzy match "rn" against repo list, assert "alpha", "alpha-core" ranked first. `TestNeedsPrompt` — various partial input states, assert which fields need prompting.

- [ ] **Implement TUI prompts for create**
  - **File(s):** `internal/tui/prompt.go`, `internal/tui/repo_selector.go`
  - `NeedsPrompt(prefilled CreateInputs) []string` — returns list of fields that need input
  - `PromptCreate(repoNames []string, prefilled CreateInputs) (CreateInputs, error)` — runs huh form for text fields, bubbletea for fuzzy repo selector
  - Fuzzy repo selector: type-to-filter, arrow keys to navigate, space to toggle, enter to confirm
  - **Makes green:** `TestPrefilledSkipsPrompt`, `TestFuzzyMatchRepos`, `TestNeedsPrompt`

- [ ] **Wire TUI into create command**
  - **File(s):** `cmd/create.go`
  - If any required input is missing from flags → call `PromptCreate` to fill gaps
  - If stdin is not a terminal (pipe/agent) and required flags are missing → error with list of missing flags
  - **Tests:** Existing `TestHeadlessCreateEndToEnd` still passes (no regression)

- [ ] **Write tests for list command logic**
  - Test listing multiple tasks from a temp tasks dir
  - Test empty tasks dir returns empty list
  - Test JSON output format
  - **File(s):** `cmd/list_test.go`
  - **Tests:** `TestListMultipleTasks` — create 2 task manifests, assert both appear with correct fields. `TestListEmpty` — empty dir, assert empty output. `TestListJSON` — assert valid JSON array output.

- [ ] **Implement list command**
  - **File(s):** `cmd/list.go`
  - Scans tasks dir for subdirectories containing `task.json`
  - Human output: table with name, created date, repo count, description (truncated)
  - JSON output: array of task summaries
  - **Makes green:** `TestListMultipleTasks`, `TestListEmpty`, `TestListJSON`

- [ ] **Write tests for info command logic**
  - Test displays full manifest for existing task
  - Test error for non-existent task
  - Test JSON output matches raw manifest
  - **File(s):** `cmd/info_test.go`
  - **Tests:** `TestInfoExistingTask` — assert all manifest fields displayed. `TestInfoNonExistent` — assert error message. `TestInfoJSON` — assert output is valid Task JSON.

- [ ] **Implement info command**
  - **File(s):** `cmd/info.go`
  - Reads `task.json` from task dir, displays formatted output
  - **Makes green:** `TestInfoExistingTask`, `TestInfoNonExistent`, `TestInfoJSON`

- [ ] **Write tests for status command logic**
  - Test shows branch and clean/dirty for each repo worktree
  - Test verbose mode shows last commit, ahead/behind
  - Test handles missing worktree gracefully (repo in manifest but dir removed)
  - **File(s):** `cmd/status_test.go`
  - **Tests:** `TestStatusCleanWorktree` — assert "clean" indicator. `TestStatusDirtyWorktree` — create uncommitted file, assert "dirty" + file count. `TestStatusVerbose` — assert commit hash and message present. `TestStatusMissingWorktree` — assert warning, not crash.

- [ ] **Implement status command**
  - **File(s):** `cmd/status.go`
  - Reads manifest, runs `git status --porcelain` in each worktree
  - Default: compact one-line-per-repo (branch, clean/dirty)
  - `--verbose`: branch, last commit (hash + message), uncommitted count, ahead/behind
  - **Makes green:** `TestStatusCleanWorktree`, `TestStatusDirtyWorktree`, `TestStatusVerbose`, `TestStatusMissingWorktree`

- [ ] **Write tests for terminal launcher**
  - Test AppleScript generation for iTerm2
  - Test AppleScript generation for Terminal.app
  - Test fallback when neither is detected
  - **File(s):** `internal/terminal/launch_test.go`
  - **Tests:** `TestITermScript` — assert generated AppleScript contains correct path. `TestTerminalAppScript` — same for Terminal.app. `TestFallbackOutput` — assert returns path string when no terminal app detected.

- [ ] **Implement terminal launcher and open command**
  - **File(s):** `internal/terminal/launch.go`, `cmd/open.go`
  - Detect iTerm2 (check if running), fall back to Terminal.app
  - Run AppleScript via `osascript -e`
  - `open` command: reads manifest, launches terminal in task dir
  - **Makes green:** `TestITermScript`, `TestTerminalAppScript`, `TestFallbackOutput`

- [ ] **Write tests for clean command logic**
  - Test removes worktrees via `git worktree remove`
  - Test removes task directory after worktree cleanup
  - Test `--archive` flag moves dir to `_archive/` instead of deleting
  - Test handles already-removed worktrees gracefully
  - **File(s):** `cmd/clean_test.go`
  - **Tests:** `TestCleanRemovesWorktrees` — assert worktree dirs gone and `git worktree list` no longer includes them. `TestCleanRemovesTaskDir` — assert task dir removed. `TestCleanArchive` — assert dir moved to `<tasksDir>/_archive/<name>`. `TestCleanPartialWorktrees` — some worktrees already gone, assert no error. `TestCleanDirtyWorktreeRejectsWithoutForce` — create uncommitted file in worktree, run clean without `--force`, assert error listing dirty repos.

- [ ] **Implement clean command**
  - **File(s):** `cmd/clean.go`
  - Flags: `--force` (skip confirmation and force-remove dirty worktrees), `--archive` (move to `_archive/` instead of deleting)
  - Reads manifest, checks each worktree for uncommitted changes
  - Without `--force`: rejects if any worktree is dirty, shows which repos have changes
  - With `--force`: passes `--force` to `git worktree remove`
  - After worktree removal, prunes (`git worktree prune` in source repo)
  - Deletes task directory (or moves to `_archive/` with `--archive`)
  - In interactive mode: confirms before proceeding. In `--json` mode: requires `--force` (no interactive confirmation possible)
  - **Makes green:** `TestCleanRemovesWorktrees`, `TestCleanRemovesTaskDir`, `TestCleanArchive`, `TestCleanPartialWorktrees`, `TestCleanDirtyWorktreeRejectsWithoutForce`

- [ ] **Write tests for add-context command logic**
  - Test copies new docs into existing task's context/
  - Test error for non-existent task
  - Test updates manifest's context_docs array
  - Test skips already-existing files with warning
  - **File(s):** `cmd/add_context_test.go`
  - **Tests:** `TestAddContextCopiesFiles` — assert files in context/. `TestAddContextNonExistentTask` — assert error. `TestAddContextUpdatesManifest` — read manifest, assert new relative paths in context_docs. `TestAddContextDuplicateFilename` — two different source paths with same filename (e.g., `/a/notes.txt` and `/b/notes.txt`), assert second is copied as `notes_1.txt` (deduplicated by destination filename). `TestAddContextAlreadyExists` — same source path added twice, assert warning and skip.

- [ ] **Implement add-context command**
  - **File(s):** `cmd/add_context.go`
  - Takes task name + one or more file paths
  - Copies files into task's context/ directory
  - Updates task.json with new context_docs entries
  - Duplicate detection is by **destination filename** (basename). If `context/notes.txt` already exists, the new file is saved as `context/notes_1.txt`. If the source path is already in the manifest's `context_docs`, it's skipped with a warning.
  - Context doc paths stored as relative paths (e.g., `context/notes.txt`)
  - **Makes green:** `TestAddContextCopiesFiles`, `TestAddContextNonExistentTask`, `TestAddContextUpdatesManifest`, `TestAddContextDuplicateFilename`, `TestAddContextAlreadyExists`

---

## Phase-Specific Risks

- **Risk 1:** Charm/bubbletea testing is model-state based, not visual. Complex fuzzy selector state transitions may be hard to test.
  - *Mitigation:* Test the fuzzy matching logic separately from the bubbletea model. Model tests verify state transitions (items selected, filter applied), not rendered output.

- **Risk 2:** iTerm2 AppleScript may vary across versions.
  - *Mitigation:* Use the well-documented iTerm2 AppleScript API. Fall back to Terminal.app, then to printing the path.

- **Risk 3:** `git worktree remove` may fail if worktree has uncommitted changes.
  - *Mitigation:* `clean` command checks for dirty worktrees and warns. `--force` flag passes `--force` to `git worktree remove`.

---

## Green Tests After This Phase

- :white_check_mark: All Phase 1 tests (no regression)
- :white_check_mark: `internal/tui` — `TestPrefilledSkipsPrompt`, `TestFuzzyMatchRepos`, `TestNeedsPrompt`
- :white_check_mark: `cmd/list` — `TestListMultipleTasks`, `TestListEmpty`, `TestListJSON`
- :white_check_mark: `cmd/info` — `TestInfoExistingTask`, `TestInfoNonExistent`, `TestInfoJSON`
- :white_check_mark: `cmd/status` — `TestStatusCleanWorktree`, `TestStatusDirtyWorktree`, `TestStatusVerbose`, `TestStatusMissingWorktree`
- :white_check_mark: `internal/terminal` — `TestITermScript`, `TestTerminalAppScript`, `TestFallbackOutput`
- :white_check_mark: `cmd/clean` — `TestCleanRemovesWorktrees`, `TestCleanRemovesTaskDir`, `TestCleanArchive`, `TestCleanPartialWorktrees`, `TestCleanDirtyWorktreeRejectsWithoutForce`
- :white_check_mark: `cmd/add_context` — `TestAddContextCopiesFiles`, `TestAddContextNonExistentTask`, `TestAddContextUpdatesManifest`, `TestAddContextDuplicateFilename`, `TestAddContextAlreadyExists`
- :x: E2E acceptance tests (binary invocation) — Phase 3

---

## Files Created/Modified

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` | Modify | Add Charm dependencies |
| `internal/tui/prompt.go` | Create | Interactive form prompts for create |
| `internal/tui/repo_selector.go` | Create | Fuzzy multi-select repo picker |
| `internal/tui/prompt_test.go` | Create | TUI logic tests |
| `internal/terminal/launch.go` | Create | iTerm2/Terminal.app launcher |
| `internal/terminal/launch_test.go` | Create | Terminal launcher tests |
| `cmd/create.go` | Modify | Wire TUI prompts into create flow |
| `cmd/list.go` | Create | List all tasks |
| `cmd/list_test.go` | Create | List command tests |
| `cmd/info.go` | Create | Show task manifest details |
| `cmd/info_test.go` | Create | Info command tests |
| `cmd/status.go` | Create | Git status across worktrees |
| `cmd/status_test.go` | Create | Status command tests |
| `cmd/open.go` | Create | Launch terminal in task dir |
| `cmd/clean.go` | Create | Tear down task workspace |
| `cmd/clean_test.go` | Create | Clean command tests |
| `cmd/add_context.go` | Create | Add context docs to task |
| `cmd/add_context_test.go` | Create | Add-context command tests |

---

**Next Step:** Once reviewed and approved, run `/para:execute --phase=2` to begin implementation.
