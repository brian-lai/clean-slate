# Summary: Clean Slate CLI — Phase 3 (Polish + Install)

**Date:** 2026-04-29
**Branch:** `para/clean-slate-cli-phase-3`
**PR:** https://github.com/brian-lai/clean-slate/pull/3
**Plan:** `context/plans/2026-04-29-clean-slate-cli-phase-3.md`

---

## What Shipped

Phase 3 turns the working CLI from Phase 2 into something you can actually `make install` and use daily: version stamping via ldflags, shell completions (zsh/bash/fish with task-name completion), a proper Makefile, and E2E acceptance tests that exercise the compiled binary as a subprocess.

```bash
make build                        # -> bin/cs
make build VERSION=v0.1.0         # -> bin/cs with v0.1.0 stamped
make install                      # installs to $GOBIN
make completions                  # -> completions/cs.{bash,zsh,fish}
make check                        # vet + gofmt + go test ./...

cs --version                      # cs version v0.1.0
cs info <TAB>                     # tab-completes task names
cs completion zsh > ~/.zsh/cs.zsh # shell completion
```

## Commits

10 commits (9 TDD pairs + 1 review fix):

1. chore: Initialize execution context for clean-slate-cli phase 3
2. Write test for version flag
3. Implement version flag with build-time ldflags injection
4. Create Makefile with build, test, install, clean targets
5. Write test for shell completion generation
6. Implement shell completion generation command
7. Write test for --json output consistency
8. Implement --json output standardization across all commands
9. Write E2E acceptance tests against compiled binary
10. Handle missing tasks dir gracefully in tab-completion (review fix)

## Tests

**87 tests, all passing.** `make check` clean (`go vet`, `gofmt`, `go test ./...`).

New Phase 3 coverage:
- `internal/version` — 1 test (default "dev" value)
- `cmd/completion` — 4 tests (zsh/bash/fish/unknown)
- `cmd/json` — 3 tests with 6 sub-tests (all-commands consistency, error shape, no stdout leaks on warnings)
- `tests/e2e` — 8 E2E tests against the compiled binary: create, list, info, status, add-context, clean, create-already-exists, --version

## Key Design Choices

- **`internal/version` as its own package** — keeps the ldflags target path stable (`github.com/blai/clean-slate/internal/version.Version`) regardless of how `cmd/` evolves.
- **`ValidArgsFunction` task-name completion** — `cs info <TAB>` reads `~/projects/tasks/` and returns names of dirs with a valid `task.json`. Skips `_archive/` and broken dirs.
- **E2E tests build once via `sync.Once`** — one `go build` per `go test ./tests/` invocation, reused across all 8 E2E cases. `testing.Short()` guard for fast unit runs.
- **Makefile `check` target** — `vet + fmt + test` in one invocation. `fmt` target actually fails if `gofmt -l` is non-empty, so `make check` is truly enforceable (not cosmetic).
- **JSON-standardization commit was doc-only** — tests proved Phase 1+2 code already met the contract. The commit adds a 12-line contract comment to `root.go` that future commands must follow.

## Deviations from Plan

- **`.gitignore` update bundled with Makefile commit** — plan listed it as a separate step; bundling was cleaner. Flagged in PR checklist.
- **JSON standardization commit did not need implementation code changes** — noted upfront in the commit message body.

## Staff+ Review

- **Round 1: APPROVED** (0 MUST FIX, 1 SHOULD FIX, 5 NIT).
  - SHOULD FIX: `cmd/completion.go` returned `ShellCompDirectiveError` when `CS_TASKS_DIR` didn't exist, surfacing as "completion error" in zsh for fresh users. Fixed in follow-up commit to return `ShellCompDirectiveNoFileComp` on ENOENT (matches `list.go`'s treatment of the same condition).
  - NITs (deferred): temp dir leak in E2E tests, dead `.PHONY lint` declaration in Makefile, wrap-then-return inconsistency in info.go, test that `open --json` doesn't launch a terminal, JSON-standardization commit name.

## Carried Forward from Phase 2 (Follow-ups Still Open)

These were explicitly deferred from Phase 2 and remain open:

- **Context doc schema migration**: `ContextDocs []string` → `[]ContextDoc{Source, Path}` to replace the `.cs-sources` sidecar. Schema-breaking.
- **Command factory pattern**: Rebuild `rootCmd` per test to remove `ResetOutput`'s flag-reset sprawl.
- **`clean` transactional safety**: if worktree removal partially fails, bail out before deleting the manifest instead of orphaning state.
- **Archive destination collision**: handle EEXIST on `os.Rename(taskDir, _archive/<name>)`.
- **Delete backward-compat wrappers** `ITermScript(dir)` / `TerminalAppScript(dir)` that now ignore `dir`.
- **Additional completion-related tests**: `ValidArgsFunction` behavior, `_archive` skip coverage.

## Learnings

- **Doc-only commits are honest when labeled.** A test commit can lock in an invariant the implementation already satisfied; the follow-up "implement" commit becomes a contract doc. Reviewers are OK with this if you're upfront about it.
- **Completion errors silently surface as shell-level ugliness.** `ShellCompDirectiveError` is a trap — it shows up as `_cs: completion error` the first time a user hits TAB on a fresh install. Always prefer `ShellCompDirectiveNoFileComp` for "no results."
- **`sync.Once`-based E2E fixtures save time.** 8 E2E tests × 1-second build each = 8s. With `sync.Once`, it's 1s.
- **`make check` as a CI contract.** Having one target that enforces vet + fmt + test means PRs never ship with formatting drift. The `fmt` target with an explicit exit 1 is what makes this real (not aspirational).
