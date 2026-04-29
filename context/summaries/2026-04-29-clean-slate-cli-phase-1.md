# Summary: Clean Slate CLI — Phase 1 (Foundation)

**Date:** 2026-04-29
**Branch:** `para/clean-slate-cli-phase-1`
**PR:** https://github.com/brian-lai/clean-slate/pull/1
**Plan:** `context/plans/2026-04-29-clean-slate-cli-phase-1.md`

---

## What Shipped

Phase 1 delivers the foundation of the `cs` CLI: the Go module, four internal packages (config, manifest, workspace, git), and a headless `create` subcommand that works end-to-end.

```
cs create my-task \
  --description "Investigate login regression" \
  --jira PROJ-549 \
  --repo alpha \
  --repo alpha-core \
  --context-doc /tmp/error.log \
  --json
```

Produces: `~/projects/tasks/my-task/` with `task.json`, `context/error.log`, and two git worktrees (`alpha/`, `alpha-core/`) on new branches `ws/my-task` branched from each repo's default branch.

## Commits (TDD: red → green, test-first for every implementation)

14 commits on the phase branch:

1. Initialize Go module and project scaffold
2. Write tests for config package
3. Implement config package
4. Write tests for manifest package
5. Implement manifest package
6. Write tests for workspace package
7. Implement workspace package
8. Write tests for git package
9. Implement git package
10. Write integration test for core create orchestration
11. Write command-level test for create subcommand
12. Implement `create` subcommand (headless mode)
13. Address PR #1 review feedback (consolidated review-fix commit)
14. Silence cobra's default error printing so JSON mode stderr stays clean

## Tests

**33 tests, all passing.** `gofmt`, `go vet`, `go test ./...` all clean.

Coverage by package:
- `internal/config` — 3 tests (defaults, env overrides, `~` expansion)
- `internal/manifest` — 8 tests (round-trip, write/read, all 4 sentinel errors, invalid name chars)
- `internal/workspace` — 7 tests (creation, context doc copy, `ErrTaskExists`, invalid name, duplicate filename collision, missing source)
- `internal/git` — 8 tests (symbolic-ref path, main/master fallback, `ErrNoDefaultBranch`, worktree add/remove, `ErrWorktreeBranchExists`, `ListRepos` filtering)
- `internal/` (integration) — 1 test (full orchestration against temp dirs)
- `cmd/` — 5 tests (headless create, JSON contract, missing flag, no-side-effects-on-validation-failure, manifest-matches-disk)

## Architecture Decisions Realized

- **Bare functions over interfaces** — no premature abstraction; interfaces deferred until mocking is needed.
- **Sentinel errors live in the owning package** — `workspace.ErrTaskExists`, `git.ErrWorktreeBranchExists`, `manifest.ErrInvalidTaskName`. One-way dependency (workspace → manifest for name validation; git → nothing outside stdlib).
- **Manifest as source of truth** — `task.json` only records context docs that actually landed on disk. `workspace.Create` returns the list of successfully-copied relative paths.
- **Fail fast + roll back on post-side-effect failure** — pre-validate name + description before any mkdir; if `manifest.Write` fails after worktrees exist, remove them and the task dir.
- **Relative paths for portability** — `context_docs` stored relative to the task dir.

## Deviations from the Plan

- **`DefaultBranch` fallback chain** — plan said `origin/HEAD → main → master → ErrNoDefaultBranch`; first impl added a fourth `abbrev-ref HEAD` fallback. Staff+ review flagged this as silent drift. Removed in review-fix commit; now matches the plan exactly.
- **Task name regex** — plan said `[a-zA-Z0-9._-]+`; reviewer flagged that leading `-` and `.` create UX footguns. Tightened to `[a-zA-Z0-9][a-zA-Z0-9._-]*` (must start alphanumeric).
- **Duplicate context-doc handling** — plan was silent on collision. First impl used `filepath.Base` and silently overwrote. Now detects collisions and renames (`notes.txt` → `notes_1.txt`) with a warning.

## Staff+ Review

- **Plan review:** 2 rounds. Round 1 found 4 MUST FIX + 8 SHOULD FIX (all about interface/signature mismatches, TDD ordering, missing sentinel errors, absolute vs. relative paths in the manifest). Round 2 approved with 1 SHOULD FIX + 1 NIT folded in.
- **PR review:** 2 rounds. Round 1 found 4 MUST FIX + 11 SHOULD FIX + 6 NIT (manifest/disk divergence, duplicate filename overwrite, no rollback on partial failure, sentinels in the wrong package, and several test/coverage gaps). All MUST FIX addressed. Key SHOULD FIX addressed: error sentinel coverage, duplicated regex, fragile substring matching in git, default-branch coverage gaps, gofmt cleanliness. Round 2: APPROVED. 2 remaining SHOULD FIX items deferred to Phase 2 (rollback test seam, double-error-printing fixed as a small follow-up commit).

## Deferred to Phase 2

- Command factory (`NewRootCmd`) to replace package-global `rootCmd` — unlocks `t.Parallel()` and removes the brittle `ResetOutput` flag reset.
- Test seam for the rollback path (test that `manifest.Write` failure after N worktrees are created removes all N + the task dir).
- `cs info` / `cs list` / `cs status` / `cs open` / `cs clean` / `cs add-context` commands.
- Interactive TUI prompts (charm/huh + bubbletea fuzzy repo selector).
- Terminal launcher (iTerm2/Terminal.app via osascript).

## Files Created

```
clean-slate/
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   ├── root.go           # Cobra root + --json + test seams (SetOutput/ResetOutput/ExecuteArgs)
│   ├── create.go         # create subcommand with pre-validation + rollback
│   └── create_test.go    # 5 command-level tests
└── internal/
    ├── config/
    │   ├── config.go     # Path config with env overrides + ~ expansion
    │   └── config_test.go
    ├── manifest/
    │   ├── manifest.go   # Task/RepoRef + Read/Write/Validate + ValidateName + sentinels
    │   └── manifest_test.go
    ├── workspace/
    │   ├── workspace.go  # Create with copy + dedupe + ErrTaskExists
    │   └── workspace_test.go
    ├── git/
    │   ├── worktree.go   # DefaultBranch + AddWorktree + RemoveWorktree + ListRepos + sentinels
    │   └── worktree_test.go
    └── integration_test.go  # Full orchestration end-to-end
```

## Learnings

- **Pre-validation is cheap insurance.** The review caught that post-side-effect validation creates orphan state. Moving validation to fail-fast at the top of `runCreate` means the command is safe to retry without manual cleanup.
- **Cobra's default error printing fights JSON output.** `SilenceErrors: true` is essential for any command that emits structured error JSON.
- **Package-global `rootCmd` makes tests coupled.** Fine for Phase 1, but will need a factory function before Phase 2 adds 6 more commands.
- **`filepath.Base` collisions are silent data loss without active detection.** The user told me the reviewer was right to catch this — duplicate filenames from different sources should never overwrite.
