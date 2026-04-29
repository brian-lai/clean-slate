# Phase 1: Foundation — Project Scaffold, Manifest, and Headless `create`

> **Parent plan:** `2026-04-29-clean-slate-cli.md`
> **Prerequisite:** None
> **Outcome:** `cs create <name> --description "..." --repo rna --repo rna-cdc` works end-to-end, creating a task directory with git worktrees and a task.json manifest. No interactive prompts yet.

---

## Objective

Establish the Go module, core internal packages (config, manifest, workspace, git), and a working `create` subcommand that accepts all inputs via flags. This phase delivers all the core logic that both the interactive TUI and headless agent will share.

---

## Key Context from Master Plan

**Relevant principles:**
- Manifest as source of truth — every task directory contains a `task.json` that fully describes the workspace
- Idempotent where possible — `create` for an existing task fails cleanly, no corruption
- Agent-friendly output — `--json` flag produces machine-parseable JSON to stdout

**Relevant architecture decisions:**
- Cobra for CLI framework — subcommand support, flag parsing
- `ws/<task-name>` branch naming — worktrees created on a new branch from repo's default branch
- JSON manifest (`task.json`) — human-readable, no schema library needed
- Default branch detection falls back: `origin/HEAD` → `main` → `master` → error

**Contracts this phase implements:**

All packages expose bare functions (not interfaces). Interfaces are deferred until a second implementation or mock is needed.

```go
// --- Sentinel errors (shared across packages) ---

var ErrTaskExists = errors.New("task already exists")
var ErrWorktreeBranchExists = errors.New("worktree branch already exists")
var ErrNoDefaultBranch = errors.New("could not detect default branch")
var ErrInvalidTaskName = errors.New("invalid task name")

// --- manifest package ---

type Task struct {
    Name        string     `json:"name"`
    CreatedAt   time.Time  `json:"created_at"`
    JiraTicket  string     `json:"jira_ticket,omitempty"`
    Description string     `json:"description"`
    Repos       []RepoRef  `json:"repos"`
    ContextDocs []string   `json:"context_docs"`  // paths relative to task dir
}

type RepoRef struct {
    Name           string `json:"name"`
    Source         string `json:"source"`
    WorktreePath   string `json:"worktree_path"`
    WorktreeBranch string `json:"worktree_branch"`
    BaseBranch     string `json:"base_branch"`
}

func Write(task Task, dir string) error
func Read(dir string) (Task, error)
func Validate(task Task) error  // checks name, description, task name chars, repo refs

// --- workspace package ---

func Create(tasksDir, taskName string, contextPaths []string) (taskDir string, warnings []string, err error)
// Returns ErrTaskExists if dir already exists. Returns ErrInvalidTaskName for bad chars.

// --- git package ---

func DefaultBranch(repoPath string) (string, error)  // fallback: origin/HEAD → main → master → ErrNoDefaultBranch
func AddWorktree(repoPath, destPath, branchName string) (baseBranch string, err error)  // returns ErrWorktreeBranchExists if branch taken
func RemoveWorktree(worktreePath string) error
func ListRepos(reposDir string) ([]string, error)  // filters to dirs with .git
```

**Testing strategy for `cmd/` package:** Command tests create real temp directories and git repos (integration-level). Commands are tested by calling `rootCmd.SetArgs(...)` + `rootCmd.Execute()` with stdout/stderr captured to `bytes.Buffer`. The `CS_TASKS_DIR` and `CS_REPOS_DIR` env vars are set to temp paths.

---

## Scope

### In Scope

- Go module initialization (`go.mod`)
- `internal/config` — base paths (tasks dir, repos dir) with overrides via env vars
- `internal/manifest` — Task struct, Read/Write/Validate functions
- `internal/workspace` — directory creation, context doc copying
- `internal/git` — worktree add, worktree remove, default branch detection
- `cmd/root.go` — root Cobra command with `--json` global flag
- `cmd/create.go` — `create` subcommand with all flags (headless mode only)
- Unit tests for all internal packages
- Integration test for the full headless `create` flow

### Out of Scope

- Interactive TUI prompts — Phase 2
- `list`, `info`, `status`, `open`, `clean`, `add-context` commands — Phase 2
- Shell completions, Makefile, install script — Phase 3

---

## Implementation Steps

> Each checklist item below maps to one git commit. The checkbox text is the commit message.
> Tests come BEFORE the implementation they cover (TDD).

- [ ] **Initialize Go module and project scaffold**
  - `go mod init github.com/blai/clean-slate`
  - Create directory structure: `cmd/`, `internal/config/`, `internal/manifest/`, `internal/workspace/`, `internal/git/`
  - Create `main.go` with minimal Cobra root command
  - Add Cobra dependency: `go get github.com/spf13/cobra`
  - **File(s):** `go.mod`, `go.sum`, `main.go`, `cmd/root.go`
  - **Tests:** None (scaffold only — verify with `go build`)

- [ ] **Write tests for config package**
  - Test default paths resolve to `~/projects/tasks` and `~/projects/repos`
  - Test env var overrides (`CS_TASKS_DIR`, `CS_REPOS_DIR`)
  - Test path expansion (`~` → home dir)
  - **File(s):** `internal/config/config_test.go`
  - **Tests:** `TestDefaultPaths` — asserts tasks/repos dirs default to `$HOME/projects/tasks` and `$HOME/projects/repos`. `TestEnvVarOverride` — sets env vars, asserts config picks them up. `TestTildeExpansion` — asserts `~/foo` expands to `$HOME/foo`.

- [ ] **Implement config package**
  - **File(s):** `internal/config/config.go`
  - Struct with `TasksDir`, `ReposDir` fields
  - `Load()` function: reads env vars, falls back to defaults, expands `~`
  - **Makes green:** `TestDefaultPaths`, `TestEnvVarOverride`, `TestTildeExpansion`

- [ ] **Write tests for manifest package**
  - Test Task struct serializes to/from JSON correctly
  - Test Write creates file, Read reads it back
  - Test Validate catches missing name, missing description
  - Test Read returns clear error for non-existent file
  - **File(s):** `internal/manifest/manifest_test.go`
  - **Tests:** `TestTaskRoundTrip` — marshal → unmarshal, assert equality; verify context_docs stored as relative paths. `TestWriteAndRead` — write to temp dir, read back, assert fields match. `TestValidateMissingName` — empty name returns `ErrInvalidTaskName`. `TestValidateMissingDescription` — empty description returns error. `TestValidateInvalidTaskNameChars` — names with spaces, colons, `..` return `ErrInvalidTaskName`; valid names `[a-zA-Z0-9._-]` pass. `TestValidateEmptyRepoName` — RepoRef with empty Name returns error. `TestReadNonExistent` — returns `os.ErrNotExist`-wrapped error.

- [ ] **Implement manifest package**
  - **File(s):** `internal/manifest/manifest.go`
  - `Task` and `RepoRef` structs (as defined in contract)
  - `Write(task Task, dir string) error` — writes `task.json` to dir. Context doc paths are stored relative to the task dir.
  - `Read(dir string) (Task, error)` — reads `task.json` from dir. Callers resolve relative context_docs paths against the task dir.
  - `Validate(task Task) error` — checks required fields, task name character validation `[a-zA-Z0-9._-]`, RepoRef.Name and RepoRef.Source non-empty
  - **Makes green:** `TestTaskRoundTrip`, `TestWriteAndRead`, `TestValidateMissingName`, `TestValidateMissingDescription`, `TestValidateInvalidTaskNameChars`, `TestValidateEmptyRepoName`, `TestReadNonExistent`

- [ ] **Write tests for workspace package**
  - Test creates task directory and context subdirectory
  - Test copies context docs into context/
  - Test fails if task directory already exists
  - Test handles empty context paths (creates dir but no files)
  - Test skips non-existent context doc paths with warning
  - **File(s):** `internal/workspace/workspace_test.go`
  - **Tests:** `TestCreateTaskDir` — creates dir at expected path, context/ subdir exists. `TestCopyContextDocs` — file content matches source. `TestAlreadyExists` — `errors.Is(err, ErrTaskExists)`. `TestInvalidTaskName` — `errors.Is(err, ErrInvalidTaskName)` for names with spaces/colons. `TestEmptyContextPaths` — context/ dir created, no files inside. `TestNonExistentContextDoc` — skips missing file, returns warnings slice.

- [ ] **Implement workspace package**
  - **File(s):** `internal/workspace/workspace.go`
  - `Create(tasksDir, taskName string, contextPaths []string) (taskDir string, warnings []string, err error)`
  - Creates `<tasksDir>/<taskName>/` and `<tasksDir>/<taskName>/context/`
  - Copies each context doc into context/
  - Returns warnings for skipped files
  - **Makes green:** `TestCreateTaskDir`, `TestCopyContextDocs`, `TestAlreadyExists`, `TestInvalidTaskName`, `TestEmptyContextPaths`, `TestNonExistentContextDoc`

- [ ] **Write tests for git package**
  - Test DefaultBranch detects default branch from a real git repo (test fixture)
  - Test DefaultBranch fallback chain (origin/HEAD → main → master)
  - Test Add creates worktree at target path with correct branch
  - Test Add fails gracefully when branch already exists
  - Test Remove cleans up worktree
  - **File(s):** `internal/git/worktree_test.go`
  - **Tests:** `TestDefaultBranch` — init a temp repo with `main` branch, assert detection. `TestDefaultBranchFallback` — repo with only `master`, assert falls back. `TestWorktreeAdd` — add worktree, verify dir exists and branch is `ws/<name>`. `TestWorktreeAddBranchExists` — pre-create branch, assert error. `TestWorktreeRemove` — add then remove, verify dir gone. `TestListRepos` — temp dir with 2 git repos and 1 non-git dir, assert only git repos returned.

- [ ] **Implement git package**
  - **File(s):** `internal/git/worktree.go`
  - `DefaultBranch(repoPath string) (string, error)` — detect via `git symbolic-ref`, fall back to `main`/`master`
  - `AddWorktree(repoPath, destPath, branchName string) (baseBranch string, err error)` — runs `git worktree add <dest> -b <branch> <base>`
  - `RemoveWorktree(worktreePath string) error` — runs `git worktree remove <path>`
  - `ListRepos(reposDir string) ([]string, error)` — lists directories in reposDir that are git repos
  - **Makes green:** `TestDefaultBranch`, `TestDefaultBranchFallback`, `TestWorktreeAdd`, `TestWorktreeAddBranchExists`, `TestWorktreeRemove`, `TestListRepos`

- [ ] **Write integration test for core create orchestration**
  - Set up temp dirs for tasks and repos, init 2 test git repos with commits on `main`
  - Calls internal packages directly (config.Load → workspace.Create → git.AddWorktree → manifest.Write), NOT the Cobra command. This tests the core orchestration logic independent of CLI flag parsing.
  - Assert: task dir created, worktrees exist with correct branches, task.json is valid and complete, context docs copied, context_docs paths are relative to task dir
  - **File(s):** `internal/integration_test.go`
  - **Tests:** `TestCreateOrchestrationEndToEnd` — full create flow in temp dirs. Asserts task.json fields, worktree dirs exist, context files present, `git branch` in worktree shows `ws/<name>`. Uses `CS_TASKS_DIR`/`CS_REPOS_DIR` env vars pointing to temp dirs.
  - **Makes green:** `TestCreateOrchestrationEndToEnd` (green immediately — all internal packages are implemented)

- [ ] **Write command-level test for create subcommand**
  - Tests the Cobra command wiring via `rootCmd.SetArgs(...)` + `rootCmd.Execute()` with stdout captured to `bytes.Buffer`
  - Set `CS_TASKS_DIR`/`CS_REPOS_DIR` to temp dirs
  - **File(s):** `cmd/create_test.go`
  - **Tests:** `TestCreateCommandHeadless` — invoke `cs create test-task --description "test" --repo <repo>`, assert exit 0, task dir created, manifest valid. `TestCreateCommandJSON` — same with `--json`, assert stdout is valid JSON. `TestCreateCommandMissingDescription` — omit required flag, assert error.

- [ ] **Implement `create` subcommand (headless mode)**
  - **File(s):** `cmd/create.go`
  - Positional arg: task name (required)
  - Flags: `--jira`, `--description`, `--repo` (repeatable), `--context-doc` (repeatable), `--json`
  - Wires together config → workspace.Create → git.AddWorktree (for each repo) → manifest.Write
  - On success: prints human-readable summary or JSON
  - On partial failure (some worktrees failed): prints warnings, still writes manifest with successful repos
  - **Makes green:** `TestCreateCommandHeadless`, `TestCreateCommandJSON`, `TestCreateCommandMissingDescription`

---

## Phase-Specific Risks

- **Risk 1:** Git worktree commands may behave differently across Git versions (e.g., `--orphan` flag availability).
  - *Mitigation:* Use the basic `git worktree add <path> -b <branch> <base>` form which is stable since Git 2.5. Test against the Git version on the dev machine.

- **Risk 2:** Default branch detection via `git symbolic-ref refs/remotes/origin/HEAD` fails if remote HEAD hasn't been set (common for freshly cloned repos).
  - *Mitigation:* Fallback chain: try `origin/HEAD`, then check if `main` branch exists, then `master`. Clear error if none found.

- **Risk 3:** `~/projects/repos/` contains non-git directories or broken symlinks.
  - *Mitigation:* `ListRepos` filters to directories that contain a `.git` dir or file. Skip others silently.

---

## Green Tests After This Phase

- :white_check_mark: `internal/config` — `TestDefaultPaths`, `TestEnvVarOverride`, `TestTildeExpansion`
- :white_check_mark: `internal/manifest` — `TestTaskRoundTrip`, `TestWriteAndRead`, `TestValidateMissingName`, `TestValidateMissingDescription`, `TestValidateInvalidTaskNameChars`, `TestValidateEmptyRepoName`, `TestReadNonExistent`
- :white_check_mark: `internal/workspace` — `TestCreateTaskDir`, `TestCopyContextDocs`, `TestAlreadyExists`, `TestInvalidTaskName`, `TestEmptyContextPaths`, `TestNonExistentContextDoc`
- :white_check_mark: `internal/git` — `TestDefaultBranch`, `TestDefaultBranchFallback`, `TestWorktreeAdd`, `TestWorktreeAddBranchExists`, `TestWorktreeRemove`, `TestListRepos`
- :white_check_mark: `internal/integration_test.go` — `TestCreateOrchestrationEndToEnd`
- :white_check_mark: `cmd/create_test.go` — `TestCreateCommandHeadless`, `TestCreateCommandJSON`, `TestCreateCommandMissingDescription`
- :x: E2E acceptance test (binary invocation) — Phase 3
- :x: TUI component tests — Phase 2

---

## Files Created/Modified

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` | Create | Go module definition |
| `main.go` | Create | Entry point |
| `cmd/root.go` | Create | Root Cobra command, `--json` global flag |
| `cmd/create.go` | Create | `create` subcommand with flags |
| `internal/config/config.go` | Create | Path configuration |
| `internal/config/config_test.go` | Create | Config unit tests |
| `internal/manifest/manifest.go` | Create | task.json read/write/validate |
| `internal/manifest/manifest_test.go` | Create | Manifest unit tests |
| `internal/workspace/workspace.go` | Create | Directory creation, file copying |
| `internal/workspace/workspace_test.go` | Create | Workspace unit tests |
| `internal/git/worktree.go` | Create | Git worktree operations |
| `internal/git/worktree_test.go` | Create | Git worktree unit tests |
| `internal/integration_test.go` | Create | End-to-end orchestration test (internal packages) |
| `cmd/create_test.go` | Create | Cobra command-level create tests |

---

**Next Step:** Once reviewed and approved, run `/para:execute --phase=1` to begin implementation.
