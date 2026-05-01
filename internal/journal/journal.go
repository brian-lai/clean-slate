// Package journal records in-flight cs operations so an abandoned operation
// (e.g., a killed process) can be rolled back on the next invocation. Every
// operation that creates worktrees writes a journal before the side effects
// and clears it after success.
//
// Journal files live at `<taskDir>/.cs-journal.<pid>` — per-PID so multiple
// concurrent cs processes never corrupt each other's journal even before the
// Phase 2 lock is wired in. ScanOrphans globs `.cs-journal.*` to discover
// journals from any PID and filters by process liveness.
package journal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brian-lai/clean-slate/internal/atomicio"
	"github.com/brian-lai/clean-slate/internal/git"
)

// Entry is the on-disk journal record, serialized as JSON.
type Entry struct {
	Op        string      `json:"op"`
	PID       int         `json:"pid"`
	Started   time.Time   `json:"started"`
	TaskDir   string      `json:"task_dir"`
	Worktrees []string    `json:"worktrees"`
	Branches  []BranchRef `json:"branches"`
}

// BranchRef names a branch that should be deleted during rollback.
type BranchRef struct {
	RepoPath string `json:"repo_path"`
	Branch   string `json:"branch"`
}

func journalPath(taskDir string, pid int) string {
	return filepath.Join(taskDir, fmt.Sprintf(".cs-journal.%d", pid))
}

// Write overwrites `<taskDir>/.cs-journal.<e.PID>` with e via atomicio.
func Write(taskDir string, e Entry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("journal: marshal entry: %w", err)
	}
	return atomicio.WriteFile(journalPath(taskDir, e.PID), data, 0644)
}

// Clear removes `<taskDir>/.cs-journal.<pid>`. No-op if the file does not exist.
func Clear(taskDir string, pid int) error {
	err := os.Remove(journalPath(taskDir, pid))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("journal: clear: %w", err)
	}
	return nil
}

// Read returns the journal entry at `<taskDir>/.cs-journal.<pid>`, a
// present-flag, and any read/parse error. (false, nil) means "no journal
// for that PID on disk" (the happy post-clear state).
func Read(taskDir string, pid int) (Entry, bool, error) {
	data, err := os.ReadFile(journalPath(taskDir, pid))
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, false, nil
		}
		return Entry{}, false, fmt.Errorf("journal: read: %w", err)
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, false, fmt.Errorf("journal: parse: %w", err)
	}
	return e, true, nil
}

// ScanOrphans globs `.cs-journal.*` under each top-level task dir in tasksDir
// and returns entries whose PID is no longer alive. Process liveness uses
// `os.FindProcess(pid).Signal(syscall.Signal(0))`. Results are sorted by
// Started ascending for deterministic recovery order.
func ScanOrphans(tasksDir string) ([]Entry, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("journal: readdir tasksDir: %w", err)
	}

	var orphans []Entry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		taskDir := filepath.Join(tasksDir, e.Name())
		matches, err := filepath.Glob(filepath.Join(taskDir, ".cs-journal.*"))
		if err != nil {
			continue
		}
		for _, path := range matches {
			base := filepath.Base(path)
			pidStr := strings.TrimPrefix(base, ".cs-journal.")
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			entry, present, err := Read(taskDir, pid)
			if err != nil || !present {
				continue
			}
			if !isAlive(pid) {
				orphans = append(orphans, entry)
			}
		}
	}

	sort.Slice(orphans, func(i, j int) bool {
		return orphans[i].Started.Before(orphans[j].Started)
	})
	if orphans == nil {
		orphans = []Entry{}
	}
	return orphans, nil
}

// isAlive reports whether the given PID is a live process. Uses kill(pid, 0)
// semantics via os.FindProcess and Signal(0). Non-positive PIDs are treated
// as dead (PID 0 is the scheduler; negative values are filenames like
// `.cs-journal.-1` that should never legitimately exist but shouldn't be
// interpreted as a live process if they do).
//
// EPERM handling: if we lack permission to signal a foreign-owned process,
// the process DOES exist — treat as alive. Otherwise we'd sweep a live
// foreign cs's work mid-operation on a multi-user tasks dir. Only ESRCH
// (or the Go "process already finished" wrapping) means dead.
// branchExists reports whether a local branch with the given name exists in
// the repo. Shells to git — no cached state, so concurrent deletions are
// observed correctly.
func branchExists(repoPath, branch string) bool {
	c := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	c.Dir = repoPath
	return c.Run() == nil
}

func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

// Rollback removes the worktrees, deletes the ws/-prefixed branches, and
// removes the task directory referenced by e. Best-effort: individual failures
// become warnings but the sweep continues. Returns a joined error if any step
// failed; otherwise nil.
func Rollback(e Entry) error {
	var failures []string

	for _, wt := range e.Worktrees {
		if _, err := os.Stat(wt); os.IsNotExist(err) {
			continue
		}
		if err := git.RemoveWorktree(wt); err != nil {
			failures = append(failures, fmt.Sprintf("remove worktree %s: %v", wt, err))
		}
	}

	for _, br := range e.Branches {
		if !strings.HasPrefix(br.Branch, "ws/") {
			continue
		}
		// Rollback is idempotent: if a concurrent sweeper already deleted the
		// branch (or the journal records a branch that was never fully
		// created), silently treat that as success rather than emitting a
		// bogus "partial recovery" warning.
		if !branchExists(br.RepoPath, br.Branch) {
			continue
		}
		if err := git.DeleteBranch(br.RepoPath, br.Branch); err != nil {
			// Even here, if git reports "not found" after our pre-check (race),
			// treat as success — the post-condition we care about is met.
			if strings.Contains(err.Error(), "not found") {
				continue
			}
			failures = append(failures, fmt.Sprintf("delete branch %s in %s: %v", br.Branch, br.RepoPath, err))
		}
	}

	if e.TaskDir != "" {
		if err := os.RemoveAll(e.TaskDir); err != nil {
			failures = append(failures, fmt.Sprintf("remove task dir %s: %v", e.TaskDir, err))
		}
	}

	if len(failures) > 0 {
		return errors.New("journal.Rollback partial failures: " + strings.Join(failures, "; "))
	}
	return nil
}
