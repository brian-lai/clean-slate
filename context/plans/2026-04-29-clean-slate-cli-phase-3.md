# Phase 3: Polish — Shell Completions, JSON Audit, Build, Install

> **Parent plan:** `2026-04-29-clean-slate-cli.md`
> **Prerequisite:** Phase 2 merged (all commands functional)
> **Outcome:** `cs` is installable via `go install` or `make install`, has shell completions for zsh/bash/fish, consistent `--json` output across all commands, and E2E acceptance tests pass against the compiled binary.

---

## Objective

Harden the CLI for daily use: add shell completions (especially zsh for the user's shell), audit every command for consistent `--json` output, create a Makefile for building/installing, and write E2E acceptance tests that invoke the compiled binary.

---

## Key Context from Master Plan

**Relevant principles:**
- Agent-friendly output — `--json` must be consistent and reliable across every command
- Interactive-first, headless-capable — shell completions improve the interactive experience

**Relevant architecture decisions:**
- Cobra has built-in shell completion generation
- Binary name is `cs`

---

## Scope

### In Scope

- Shell completion generation (zsh, bash, fish) via Cobra's built-in support
- `--json` output audit: ensure every command outputs valid, structured JSON when flag is set
- Makefile: `build`, `test`, `install`, `clean` targets
- E2E acceptance tests: invoke compiled `cs` binary, assert exit codes and output
- Version flag (`cs --version`) with build-time injection via ldflags
- `.gitignore` updates for build artifacts

### Out of Scope

- Homebrew formula (future — once repo is public)
- Cross-platform builds (Linux, Windows — future)
- CI/CD pipeline (future)

---

## Implementation Steps

> Each checklist item below maps to one git commit. The checkbox text is the commit message.
> Tests come BEFORE the implementation they cover (TDD).

- [ ] **Write test for version flag**
  - **File(s):** `internal/version/version_test.go`
  - **Tests:** `TestVersionDefault` — asserts Version is "dev" when not injected via ldflags

- [ ] **Implement version flag with build-time ldflags injection**
  - **File(s):** `internal/version/version.go`, `cmd/root.go`
  - Add `var Version = "dev"` in version package, injected via `-ldflags "-X ...Version=v0.1.0"` at build time
  - Add `--version` flag to root command
  - **Makes green:** `TestVersionDefault`

- [ ] **Create Makefile with build, test, install, clean targets**
  - **File(s):** `Makefile`
  - `build`: `go build -ldflags "-X .../version.Version=$(VERSION)" -o bin/cs`
  - `test`: `go test ./...`
  - `install`: `go install` with ldflags
  - `clean`: remove `bin/`
  - `completions`: generate completion files for zsh/bash/fish into `completions/`
  - **Tests:** None (build system — verify with `make build && ./bin/cs --version`)

- [ ] **Write test for shell completion generation**
  - **File(s):** `cmd/completion_test.go`
  - **Tests:** `TestCompletionZsh` — assert output contains expected zsh completion markers. `TestCompletionBash` — assert output contains bash completion markers.

- [ ] **Implement shell completion generation command**
  - **File(s):** `cmd/completion.go`
  - `cs completion zsh`, `cs completion bash`, `cs completion fish`
  - Uses Cobra's `GenZshCompletion`, `GenBashCompletion`, `GenFishCompletion`
  - Register `ValidArgsFunction` on commands that take task names (info, status, open, clean, add-context) — scans tasks dir and returns names for tab completion
  - Include instructions in output for how to install (e.g., "Add to your .zshrc: source <(cs completion zsh)")
  - **Makes green:** `TestCompletionZsh`, `TestCompletionBash`

- [ ] **Write test for --json output consistency**
  - **File(s):** `cmd/json_test.go`
  - **Tests:** `TestJSONOutputConsistency` — invoke each command's run logic with `--json`, assert all outputs parse as valid JSON. `TestJSONErrorFormat` — trigger errors (non-existent task, invalid name), assert JSON error format `{"error": "message"}`. `TestJSONNoStdoutLeaks` — with `--json`, assert no non-JSON text on stdout (warnings must go to stderr).

- [ ] **Implement --json output standardization across all commands**
  - **File(s):** `cmd/root.go`, all `cmd/*.go` files
  - Extract `OutputJSON(w io.Writer, v any) error` helper in root
  - Extract `OutputError(w io.Writer, err error) error` for consistent error JSON
  - Ensure every command: (1) checks `--json` flag, (2) outputs valid JSON, (3) uses consistent error JSON format `{"error": "message"}`
  - Ensure no human-readable text leaks into stdout when `--json` is set (warnings go to stderr)
  - **Makes green:** `TestJSONOutputConsistency`, `TestJSONErrorFormat`, `TestJSONNoStdoutLeaks`

- [ ] **Write E2E acceptance tests against compiled binary**
  - Build binary via `go build`, invoke via `exec.Command` in tests
  - Each test sets `CS_TASKS_DIR` and `CS_REPOS_DIR` env vars to temp directories
  - **File(s):** `tests/e2e_test.go`
  - **Tests:**
    - `TestE2ECreateHeadless` — `cs create test-task --description "test" --repo <test-repo> --json`, assert exit 0, valid JSON, task dir exists
    - `TestE2EList` — create task, `cs list --json`, assert task appears
    - `TestE2EInfo` — `cs info test-task --json`, assert manifest fields
    - `TestE2EStatus` — `cs status test-task`, assert output contains repo names
    - `TestE2EAddContext` — `cs add-context test-task <file> --json`, assert file copied
    - `TestE2EClean` — `cs clean test-task --force`, assert task dir removed
    - `TestE2ECreateAlreadyExists` — `cs create test-task ...` twice, assert exit 1 and JSON error on second

- [ ] **Update .gitignore for build artifacts**
  - **File(s):** `.gitignore`
  - Add `bin/`, `completions/`, `*.test`

---

## Phase-Specific Risks

- **Risk 1:** E2E tests are slower and more brittle than unit tests (depend on compiled binary, temp dirs, real git).
  - *Mitigation:* Use `testing.Short()` to skip E2E in quick test runs. Each test creates isolated temp dirs.

- **Risk 2:** Shell completions may not work correctly with custom subcommand argument completion (e.g., completing task names for `cs info <tab>`).
  - *Mitigation:* Register Cobra `ValidArgsFunction` on commands that take task names — scans tasks dir and returns names.

---

## Green Tests After This Phase

- :white_check_mark: All Phase 1 tests (no regression)
- :white_check_mark: All Phase 2 tests (no regression)
- :white_check_mark: `internal/version` — `TestVersionDefault`
- :white_check_mark: `cmd/completion` — `TestCompletionZsh`, `TestCompletionBash`
- :white_check_mark: `cmd/` JSON audit — `TestJSONOutputConsistency`, `TestJSONErrorFormat`, `TestJSONNoStdoutLeaks`
- :white_check_mark: `tests/e2e_test.go` — all 7 E2E acceptance tests

---

## Files Created/Modified

| File | Action | Purpose |
|------|--------|---------|
| `internal/version/version.go` | Create | Version variable with ldflags injection |
| `internal/version/version_test.go` | Create | Version unit test |
| `cmd/root.go` | Modify | Add --version, OutputJSON/OutputError helpers |
| `cmd/completion.go` | Create | Shell completion generation |
| `cmd/completion_test.go` | Create | Shell completion tests |
| `cmd/json_test.go` | Create | JSON output consistency tests |
| `cmd/*.go` | Modify | Standardize --json output across all commands |
| `Makefile` | Create | Build, test, install, clean, completions targets |
| `.gitignore` | Modify | Exclude bin/, completions/, *.test |
| `tests/e2e_test.go` | Create | E2E acceptance tests |

---

**Next Step:** Once reviewed and approved, run `/para:execute --phase=3` to begin implementation.
