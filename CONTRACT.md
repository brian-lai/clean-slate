# `cs` CLI Contract (v0.2.0+)

This document is the agent-facing stable contract for `cs`. Anything here is considered load-bearing and will not change without a semver bump and release notes call-out.

## Exit codes

- `0` — success
- `1` — any error (task not found, lock held, invalid input, etc.)
- `2` — (reserved) usage error from Cobra arg-validation failures — currently also returns `1`; may narrow to `2` in a future minor.

## Output streams

In `--json` mode:
- **Success:** JSON payload on stdout; stderr is reserved for warnings (see below).
- **Error:** JSON `{"error": "<message>"}` on stderr; stdout empty.

In human mode:
- **Success:** human-readable summary on stdout.
- **Error:** `Error: <message>` on stderr.

## Per-task advisory locking (v0.2.0+)

`cs create`, `cs clean`, and `cs add-context` acquire a non-blocking advisory lock at `<tasksDir>/.cs-locks/<taskname>.lock` before mutating the task. Independent tasks proceed in parallel.

On contention (another cs process holds the lock):
- Exit code `1`.
- `--json` stderr: `{"error": "task \"<name>\" is locked by PID <pid> (started <duration> ago)"}`
- Human stderr: `Error: task "<name>" is locked by PID <pid> (started <duration> ago)`
- If the current holder wrote the payload but a reader raced its truncate window, the error reports "locked by an unknown process" instead.

Agents should treat lock-contention as retriable after a short delay. There is no built-in wait; that is the agent's policy.

## Lazy orphan recovery (v0.2.0+)

Every `cs` command opportunistically scans `<tasksDir>` for `.cs-journal.<pid>` files left by crashed `cs` processes (PID no longer alive). For each orphan, `cs` rolls back the recorded side effects (worktrees, `ws/` branches, task dir) and emits a warning:

- **Write-mutating commands** (`create`, `clean`, `add-context`): warnings appended to the JSON payload's existing `warnings` array.
- **Read-only commands** (`list`, `info`, `status`, `open`): warnings emitted to stderr as plain text `Warning: recovered orphaned task "<name>" (crashed <duration> ago)` in both human and `--json` modes. stdout JSON stays the same canonical shape.

Agents do NOT need to invoke a recovery command. There is no `cs doctor`; recovery happens transparently on the next `cs` invocation touching the tasks dir.

## File conventions (inside tasks dir)

- `<tasksDir>/<taskname>/` — user task dir (contains `task.json`, `context/`, and one subdir per repo worktree).
- `<tasksDir>/.cs-locks/<taskname>.lock` — per-task advisory lock (removed on `Release`).
- `<tasksDir>/<taskname>/.cs-journal.<pid>` — per-process write-ahead journal (removed on successful op or orphan sweep).

All files prefixed with `.cs-` are cs-owned. `cs list` filters by presence of `task.json`, so these are invisible.

## Atomic writes (v0.2.0+)

`task.json` and journal files are written via temp-file + fsync + rename + dir-fsync. A SIGKILL mid-write leaves the previous content intact. Durability against sudden power loss is filesystem-dependent (tmpfs/NFSv3 without rpc.lockd may not honor fsync).

## Platform support

- **Supported:** macOS and Linux.
- **Not supported:** Windows. The `cslock` package has a Windows stub that returns `ErrUnsupportedPlatform` from every entry point; `GOOS=windows go build ./...` succeeds for packaging/test purposes but `cs` commands that acquire the lock will fail at runtime.

## JSON output shapes (write-mutating commands)

### `cs create --json` success

```json
{
  "task": { ... full task manifest ... },
  "task_dir": "/absolute/path",
  "warnings": []
}
```

### `cs clean --json` success

```json
{
  "task": "<name>",
  "archived": false,
  "warnings": []
}
```

### `cs add-context --json` success

```json
{
  "task": "<name>",
  "added": ["context/..."],
  "warnings": []
}
```

The `warnings` array may include orphan-recovery messages as described above.

## What's NOT in the contract yet

These are deferred to future minors; don't rely on them being stable today:

- Structured error codes (`{"error": {"code": "LOCKED", ...}}`).
- `cs doctor` / manual GC command.
- Per-repo locking (git's own `index.lock` covers same-repo races today).
- `--wait` flag for lock contention.
- VCS backends other than git.
- Windows support.

## Revert safety

Every new file/dir cs writes in v0.2.0 (`.cs-locks/`, `.cs-journal.<pid>`) is inert to older cs versions — they're unknown dotfiles. Downgrading is safe as long as no cs process is mid-operation.
