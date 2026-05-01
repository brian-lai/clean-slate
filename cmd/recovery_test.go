package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brian-lai/clean-slate/internal/journal"
)

// deadPIDForRecovery spawns a subprocess, waits for it, returns its reaped PID.
func deadPIDForRecovery(t *testing.T) int {
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

// writeOrphan sets up a "ghost" task: a task dir on disk with a journal
// referencing a dead PID, plus the worktree + branch the journal records.
// No task.json is written so the task is invisible to cs list.
func writeOrphan(t *testing.T, tasksDir, reposDir, taskName, repoName string, dp int) string {
	t.Helper()
	repoDir := filepath.Join(reposDir, repoName)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initTestRepo(t, repoDir)

	taskDir := filepath.Join(tasksDir, taskName)
	if err := os.MkdirAll(filepath.Join(taskDir, "context"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create the worktree + ws/ branch via plain git so the journal isn't racing.
	worktreeDest := filepath.Join(taskDir, repoName)
	branchName := "ws/" + taskName
	c := exec.Command("git", "worktree", "add", worktreeDest, "-b", branchName, "main")
	c.Dir = repoDir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}

	entry := journal.Entry{
		Op:        "create",
		PID:       dp,
		Started:   time.Now().UTC(),
		TaskDir:   taskDir,
		Worktrees: []string{worktreeDest},
		Branches:  []journal.BranchRef{{RepoPath: repoDir, Branch: branchName}},
	}
	if err := journal.Write(taskDir, entry); err != nil {
		t.Fatal(err)
	}
	return repoDir
}

// TestOrphanRecoveryOnListStartup verifies that running `cs list` sweeps
// orphaned journals, rolls them back, and emits a warning to stderr.
func TestOrphanRecoveryOnListStartup(t *testing.T) {
	tasksDir, reposDir := setupTestEnv(t, nil)
	dp := deadPIDForRecovery(t)
	if dp == os.Getpid() {
		t.Skip("dead PID collision; rerun")
	}
	repoDir := writeOrphan(t, tasksDir, reposDir, "ghost", "repo-ghost", dp)

	_, stderr, err := executeCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("list: %v (stderr: %s)", err, stderr)
	}

	if !strings.Contains(stderr, "recovered orphaned task") {
		t.Errorf("stderr missing orphan-recovery warning: %q", stderr)
	}
	if !strings.Contains(stderr, "ghost") {
		t.Errorf("stderr missing task name: %q", stderr)
	}

	// Orphan task dir should be gone.
	if _, err := os.Stat(filepath.Join(tasksDir, "ghost")); !os.IsNotExist(err) {
		t.Errorf("ghost task dir should be removed")
	}

	// ws/ghost branch should be gone from the source repo.
	out, _ := exec.Command("git", "-C", repoDir, "branch", "--list", "ws/ghost").Output()
	if len(out) != 0 {
		t.Errorf("ws/ghost branch should be deleted: %s", out)
	}
}

// TestOrphanRecoveryOnCreateEmbedsInWarnings verifies that the sweep
// warnings are embedded in the JSON success payload's `warnings` field for
// write-mutating commands rather than emitted to stderr.
func TestOrphanRecoveryOnCreateEmbedsInWarnings(t *testing.T) {
	tasksDir, reposDir := setupTestEnv(t, []string{"repo-new"})

	dp := deadPIDForRecovery(t)
	if dp == os.Getpid() {
		t.Skip("dead PID collision; rerun")
	}
	// Setup a ghost alongside a fresh create target.
	writeOrphan(t, tasksDir, reposDir, "ghost2", "repo-ghost2", dp)

	stdout, _, err := executeCmd(t, "create", "newtask",
		"--description", "fresh",
		"--repo", "repo-new",
		"--json",
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	foundOrphanWarning := false
	for _, w := range payload.Warnings {
		if strings.Contains(w, "recovered orphaned task") && strings.Contains(w, "ghost2") {
			foundOrphanWarning = true
			break
		}
	}
	if !foundOrphanWarning {
		t.Errorf("create payload warnings missing orphan recovery; got: %v", payload.Warnings)
	}
}
