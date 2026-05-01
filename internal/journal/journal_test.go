package journal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brian-lai/clean-slate/internal/journal"
)

func makeEntry(pid int, taskDir string) journal.Entry {
	return journal.Entry{
		Op:        "create",
		PID:       pid,
		Started:   time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		TaskDir:   taskDir,
		Worktrees: []string{filepath.Join(taskDir, "repo-a")},
		Branches: []journal.BranchRef{
			{RepoPath: "/projects/repos/repo-a", Branch: "ws/test"},
		},
	}
}

func TestWriteReadRoundtrip(t *testing.T) {
	taskDir := t.TempDir()
	in := makeEntry(os.Getpid(), taskDir)

	if err := journal.Write(taskDir, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, ok, err := journal.Read(taskDir, in.PID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !ok {
		t.Fatal("Read: expected journal present, got false")
	}

	if out.Op != in.Op || out.PID != in.PID || out.TaskDir != in.TaskDir {
		t.Errorf("top-level mismatch: got %+v want %+v", out, in)
	}
	if !out.Started.Equal(in.Started) {
		t.Errorf("Started mismatch: got %v want %v", out.Started, in.Started)
	}
	if len(out.Worktrees) != 1 || out.Worktrees[0] != in.Worktrees[0] {
		t.Errorf("Worktrees mismatch: got %v want %v", out.Worktrees, in.Worktrees)
	}
	if len(out.Branches) != 1 || out.Branches[0] != in.Branches[0] {
		t.Errorf("Branches mismatch: got %v want %v", out.Branches, in.Branches)
	}
}

// TestPerPIDIsolation verifies that two different PIDs writing to the same
// taskDir produce disjoint files. This is the design property that makes
// Phase 1 safely shippable standalone (before the Phase 2 lock exists).
func TestPerPIDIsolation(t *testing.T) {
	taskDir := t.TempDir()
	a := makeEntry(1001, taskDir)
	a.Op = "create-by-a"
	b := makeEntry(1002, taskDir)
	b.Op = "create-by-b"

	if err := journal.Write(taskDir, a); err != nil {
		t.Fatal(err)
	}
	if err := journal.Write(taskDir, b); err != nil {
		t.Fatal(err)
	}

	gotA, okA, err := journal.Read(taskDir, 1001)
	if err != nil || !okA {
		t.Fatalf("Read A: ok=%v err=%v", okA, err)
	}
	gotB, okB, err := journal.Read(taskDir, 1002)
	if err != nil || !okB {
		t.Fatalf("Read B: ok=%v err=%v", okB, err)
	}
	if gotA.Op != "create-by-a" {
		t.Errorf("A.Op = %q, want create-by-a", gotA.Op)
	}
	if gotB.Op != "create-by-b" {
		t.Errorf("B.Op = %q, want create-by-b", gotB.Op)
	}

	// On disk: both journal files exist.
	entries, _ := os.ReadDir(taskDir)
	var journalCount int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".cs-journal.") {
			journalCount++
		}
	}
	if journalCount != 2 {
		t.Errorf("expected 2 per-PID journals, got %d", journalCount)
	}
}

func TestReadMissingReturnsFalseNoError(t *testing.T) {
	taskDir := t.TempDir()
	e, ok, err := journal.Read(taskDir, 999)
	if err != nil {
		t.Errorf("Read missing: want nil error, got %v", err)
	}
	if ok {
		t.Error("Read missing: want ok=false, got true")
	}
	if e.PID != 0 {
		t.Errorf("Read missing: want zero-value Entry, got %+v", e)
	}
}

func TestClearIdempotent(t *testing.T) {
	taskDir := t.TempDir()
	e := makeEntry(42, taskDir)

	// Clear before Write — no error.
	if err := journal.Clear(taskDir, 42); err != nil {
		t.Errorf("Clear on missing: %v", err)
	}

	// Write then Clear.
	if err := journal.Write(taskDir, e); err != nil {
		t.Fatal(err)
	}
	if err := journal.Clear(taskDir, 42); err != nil {
		t.Errorf("Clear after Write: %v", err)
	}
	_, ok, _ := journal.Read(taskDir, 42)
	if ok {
		t.Error("journal file still present after Clear")
	}

	// Second Clear — no error.
	if err := journal.Clear(taskDir, 42); err != nil {
		t.Errorf("double Clear: %v", err)
	}
}

// TestClearIsolatedByPID verifies that Clearing one PID's journal does NOT
// touch another PID's journal in the same taskDir.
func TestClearIsolatedByPID(t *testing.T) {
	taskDir := t.TempDir()
	a := makeEntry(1001, taskDir)
	b := makeEntry(1002, taskDir)
	journal.Write(taskDir, a)
	journal.Write(taskDir, b)

	if err := journal.Clear(taskDir, 1001); err != nil {
		t.Fatal(err)
	}

	if _, ok, _ := journal.Read(taskDir, 1001); ok {
		t.Error("1001 journal should be gone")
	}
	if _, ok, _ := journal.Read(taskDir, 1002); !ok {
		t.Error("1002 journal should still exist")
	}
}

// TestWriteUsesAtomicIO — no .tmp-* siblings after Write.
func TestWriteUsesAtomicIO(t *testing.T) {
	taskDir := t.TempDir()
	if err := journal.Write(taskDir, makeEntry(7, taskDir)); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(taskDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover tempfile: %s", e.Name())
		}
	}
}

// deadPID spawns a subprocess, waits for it, and returns its PID. The kernel
// has reaped the process so that PID is guaranteed dead. Safer than hardcoding
// a large number because pid_max can be arbitrarily high on modern Linux.
func deadPID(t *testing.T) int {
	t.Helper()
	c := exec.Command("true")
	if runtime.GOOS == "darwin" {
		c = exec.Command("/usr/bin/true")
	}
	if err := c.Run(); err != nil {
		t.Fatalf("spawn dead-PID subprocess: %v", err)
	}
	return c.Process.Pid
}

func TestScanOrphansFiltersLiveProcesses(t *testing.T) {
	tasksDir := t.TempDir()
	aliveDir := filepath.Join(tasksDir, "alive")
	deadDir := filepath.Join(tasksDir, "dead")
	os.MkdirAll(aliveDir, 0755)
	os.MkdirAll(deadDir, 0755)

	dp := deadPID(t)

	// Sanity: dp must not coincidentally equal os.Getpid().
	if dp == os.Getpid() {
		t.Skip("dead PID collided with ourselves; rerun")
	}

	aliveEntry := makeEntry(os.Getpid(), aliveDir)
	deadEntry := makeEntry(dp, deadDir)

	if err := journal.Write(aliveDir, aliveEntry); err != nil {
		t.Fatal(err)
	}
	if err := journal.Write(deadDir, deadEntry); err != nil {
		t.Fatal(err)
	}

	orphans, err := journal.ScanOrphans(tasksDir)
	if err != nil {
		t.Fatalf("ScanOrphans: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d: %+v", len(orphans), orphans)
	}
	if orphans[0].PID != dp {
		t.Errorf("orphan PID = %d, want %d (dead)", orphans[0].PID, dp)
	}
}

func TestScanOrphansIgnoresMissingDirs(t *testing.T) {
	orphans, err := journal.ScanOrphans(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Errorf("ScanOrphans on missing dir: want nil error, got %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected empty slice, got %v", orphans)
	}
}

func TestScanOrphansMultiplePIDsPerTaskDir(t *testing.T) {
	tasksDir := t.TempDir()
	taskDir := filepath.Join(tasksDir, "shared")
	os.MkdirAll(taskDir, 0755)

	dp := deadPID(t)
	if dp == os.Getpid() {
		t.Skip("PID collision; rerun")
	}

	aliveEntry := makeEntry(os.Getpid(), taskDir)
	deadEntry := makeEntry(dp, taskDir)
	journal.Write(taskDir, aliveEntry)
	journal.Write(taskDir, deadEntry)

	orphans, err := journal.ScanOrphans(tasksDir)
	if err != nil {
		t.Fatalf("ScanOrphans: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected exactly 1 orphan (dead PID), got %d: %+v", len(orphans), orphans)
	}
	if orphans[0].PID != dp {
		t.Errorf("orphan PID = %d, want %d", orphans[0].PID, dp)
	}
}

// TestRollbackRemovesWorktreesAndBranches exercises the full rollback sweep
// against a real git repo + real worktree + real branch.
func TestRollbackRemovesWorktreesAndBranches(t *testing.T) {
	reposDir := t.TempDir()
	tasksDir := t.TempDir()

	repoPath := filepath.Join(reposDir, "repo-rb")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(repoPath, "init", "-b", "main")
	run(repoPath, "config", "user.email", "test@test.com")
	run(repoPath, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	run(repoPath, "add", ".")
	run(repoPath, "commit", "-m", "init")

	taskDir := filepath.Join(tasksDir, "rb-task")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}
	worktreeDest := filepath.Join(taskDir, "repo-rb")
	run(repoPath, "worktree", "add", worktreeDest, "-b", "ws/rb-task", "main")

	entry := journal.Entry{
		Op:        "create",
		PID:       os.Getpid(),
		Started:   time.Now().UTC(),
		TaskDir:   taskDir,
		Worktrees: []string{worktreeDest},
		Branches:  []journal.BranchRef{{RepoPath: repoPath, Branch: "ws/rb-task"}},
	}

	if err := journal.Rollback(entry); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if _, err := os.Stat(worktreeDest); !os.IsNotExist(err) {
		t.Errorf("worktree should be removed")
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("task dir should be removed")
	}

	listOut, _ := exec.Command("git", "-C", repoPath, "branch", "--list", "ws/rb-task").Output()
	if len(listOut) != 0 {
		t.Errorf("ws/rb-task branch should be deleted, got: %s", listOut)
	}
}

// TestRollbackIgnoresNonWsBranch verifies that Rollback does NOT touch a
// branch whose name doesn't start with ws/ — defense-in-depth against a
// tampered or corrupted journal.
func TestRollbackIgnoresNonWsBranch(t *testing.T) {
	entry := journal.Entry{
		TaskDir: t.TempDir(),
		Branches: []journal.BranchRef{
			{RepoPath: "/nonexistent", Branch: "keepme"},
		},
	}
	if err := journal.Rollback(entry); err != nil {
		t.Errorf("Rollback should ignore non-ws/ branch, got error: %v", err)
	}
}
