# Summary: Clean Slate CLI — Phase 2 (Interactive TUI + Commands)

**Date:** 2026-04-29
**Branch:** `para/clean-slate-cli-phase-2`
**PR:** https://github.com/brian-lai/clean-slate/pull/2
**Plan:** `context/plans/2026-04-29-clean-slate-cli-phase-2.md`

---

## What Shipped

Phase 2 layers interactive TUI on top of the `create` command and adds six new subcommands. Every command works in both interactive TTY and headless `--json` modes.

```
cs create                            # interactive — prompts for name, desc, repos via fuzzy selector
cs create my-task --description ...  # headless — same as Phase 1
cs list                              # table view of all tasks
cs info my-task                      # full manifest details
cs status my-task [--verbose]        # git status across all worktrees
cs open my-task                      # launch iTerm/Terminal in task dir
cs clean my-task [--force] [--archive]  # tear down worktrees + task dir
cs add-context my-task /path/to/doc.txt [...]  # add more context docs
```

## Commits (TDD: test-first for every command + review fix)

18 commits on the branch:

1. chore: Initialize execution context for clean-slate-cli phase 2
2. Add Charm dependencies (bubbletea, huh, lipgloss, bubbles, sahilm/fuzzy)
3. Write tests for TUI prompt logic
4. Implement TUI prompts for create
5. Wire TUI into create command
6. Write tests for list command logic
7. Implement list command
8. Write tests for info command logic
9. Implement info command
10. Write tests for status command logic
11. Implement status command
12. Write tests for terminal launcher
13. Implement terminal launcher and open command
14. Write tests for clean command logic
15. Implement clean command
16. Write tests for add-context command logic
17. Implement add-context command
18. Address PR #2 review feedback

## Tests

**71 tests, all passing** (up from Phase 1's 33). `gofmt`, `go vet`, `go test ./...` all clean.

New coverage in Phase 2:
- `internal/tui` — 6 tests (NeedsPrompt, fuzzy match: happy/no-match/empty-query)
- `internal/terminal` — 6 tests (argv-based templates, backward-compat wrappers, fallback shell quoting)
- `cmd/list` — 4 tests (multiple tasks, empty, JSON, JSON empty)
- `cmd/info` — 3 tests (existing task, non-existent, JSON)
- `cmd/status` — 5 tests (clean, dirty, verbose, missing worktree, non-existent task)
- `cmd/open` — 2 tests (non-existent task, JSON mode returns path)
- `cmd/clean` — 8 tests (removes worktrees, removes task dir, archive, partial, dirty rejection, non-existent, non-TTY force required, JSON force required)
- `cmd/add-context` — 5 tests (copies files, non-existent task, updates manifest, duplicate filename rename, same-source dedupe)

## Key Design Choices

- **TUI only prompts when stdin is a TTY AND `--json` is not set.** Pipe/agent/CI use fails fast with a clear "missing required input(s)" error instead of blocking on a prompt that can't be answered.
- **iTerm2 detection via `osascript` System Events query.** Falls back to Terminal.app, then to printing `cd <path>` for the user to copy.
- **AppleScript paths passed via argv, not interpolated.** `osascript -e <template> <path>` — the template reads `item 1 of argv` and uses AppleScript's `quoted form of` for shell quoting. Zero parser injection surface.
- **`clean` requires explicit confirmation in interactive mode, `--force` in non-interactive.** Dirty worktrees always need `--force`. Ctrl-C cancels cleanly.
- **`add-context` dedupes by source path** (via a hidden `.cs-sources` sidecar). Same source twice → skipped with warning. Different sources with the same basename → second renamed to `notes_1.txt`.
- **`status` handles missing worktrees gracefully.** Manifest references a worktree that's no longer on disk → reported as `missing`, doesn't crash.

## Deviations from the Plan

- **Initial AppleScript escaping was wrong.** First impl interpolated the path into AppleScript with POSIX single-quote escaping, which defends against the shell but not the AppleScript parser. Reviewer caught this as a MUST FIX; argv-based approach replaced it. Lesson: when a function claims "injection safety," the test and the escape must target the actual parser, not a downstream one.
- **`clean` was advertised as interactive but had no prompt.** Reviewer caught this; added `huh.NewConfirm`. Lesson: flag help text is a contract — if `--force` says "skip confirmation," there must BE a confirmation to skip.

## Staff+ Review

- **Round 1:** 2 MUST FIX (AppleScript injection, missing clean confirmation) + 5 SHOULD FIX (open tests, JSON tests, sidecar abstraction, go.mod directness, status error surfacing) + 5 NIT.
- **Round 2:** APPROVED. All MUST FIX resolved. Key SHOULD FIX addressed: open tests, go mod tidy, sidecar improvements. Deferred: JSON tests for status/clean/add-context, schema change for context docs (both tracked for Phase 3).

## Deferred to Phase 3 (or follow-up)

- **Context doc schema migration.** Change `ContextDocs []string` to `[]ContextDoc{Source, Path}` so source tracking lives in `task.json` instead of the `.cs-sources` sidecar. Schema-breaking, so bundle with any other task.json changes.
- **JSON-mode test coverage for `status`, `clean`, `add-context`.** Phase 1's pattern of asserting `stderr == ""`, valid JSON on stdout, and `[]` not `null` should be extended to these commands.
- **Command factory pattern.** `ResetOutput`'s per-command flag reset is now 20 lines. Phase 3 should rebuild `rootCmd` per test instead.
- **`clean` transactional safety.** If `git.RemoveWorktree` fails mid-loop, the manifest is still deleted and worktrees may be orphaned. Consider bailing out on first failure or archiving the manifest as a recovery breadcrumb.
- **Archive destination collision.** `os.Rename(taskDir, archive/<name>)` fails if the destination already exists. Add timestamp suffix or clearer error.
- **Delete the `ITermScript`/`TerminalAppScript` backward-compat wrappers.** They now ignore their `dir` argument. The original tests were rewritten, so the wrappers aren't needed — but a follow-up PR should remove them cleanly.

## Files Created

New in Phase 2:
```
cmd/
├── add_context.go, add_context_test.go
├── clean.go, clean_test.go
├── info.go, info_test.go
├── list.go, list_test.go
├── open.go, open_test.go
└── status.go, status_test.go
internal/
├── terminal/
│   ├── launch.go, launch_test.go
└── tui/
    └── prompt.go, prompt_test.go
```

Modified in Phase 2:
```
cmd/create.go   # Wire TUI for interactive mode
cmd/root.go     # Add ResetOutput entries for new flags
go.mod, go.sum  # Add Charm deps
```

## Learnings

- **"Injection safety" must be verified at the correct parser.** The reviewer's concrete AppleScript exploit (path with `"` + newline + `do shell script "..."`) forced me to re-examine the escape. Shell-level escaping is necessary but not sufficient when the string first passes through another parser.
- **Advertised behavior is part of the API contract.** `clean --force` "skip confirmation" implied a confirmation existed. If it doesn't, the help text is wrong — users will misfire the command. Flag help is tested documentation.
- **`huh` composes nicely with existing Cobra flow.** Drop-in `Confirm()` and `MultiSelect()` without needing a custom bubbletea model.
- **Sidecar files for state management are usually a mistake.** `.cs-sources` works but couples two sources of truth (manifest + sidecar). The reviewer was right that a schema change is cleaner. Deferred only because the refactor scope would have blocked Phase 2.
- **TDD caught real bugs during implementation.** `TestAddContextDuplicateFilename` failed on the first pass because the dedupe logic checked manifest entries by destination path, not source path. The test forced a redesign.
