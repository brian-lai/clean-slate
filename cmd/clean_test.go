package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brian-lai/clean-slate/internal/manifest"
)

func TestCleanRemovesWorktrees(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	_, worktreeDir := setupTaskWithWorktree(t, tasksDir, reposDir, "clean-me", "repo-clean")
	repoDir := filepath.Join(reposDir, "repo-clean")

	_, _, err := executeCmd(t, "clean", "clean-me", "--force")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}

	// Worktree directory should be gone
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after clean")
	}

	// git worktree list in the source repo should not include the removed worktree
	cmd := exec.Command("git", "worktree", "list")
	cmd.Dir = repoDir
	out, _ := cmd.Output()
	if containsString(string(out), "clean-me") {
		t.Errorf("git worktree list still shows clean-me: %s", out)
	}

	// The ws/clean-me branch should also be deleted from the source repo so that
	// repos don't accumulate abandoned task branches over time.
	branchCmd := exec.Command("git", "branch", "--list", "ws/clean-me")
	branchCmd.Dir = repoDir
	branchOut, _ := branchCmd.Output()
	if len(branchOut) != 0 {
		t.Errorf("ws/clean-me branch still exists after clean: %q", branchOut)
	}
}

// TestCleanLeavesNonWsBranch verifies that if the manifest references a branch
// that doesn't match the ws/* convention (e.g., manually tampered), clean
// leaves the branch alone rather than deleting it. cs create always writes
// ws/<name>, but we don't assume the manifest is trustworthy for destructive ops.
func TestCleanLeavesNonWsBranch(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	taskDir, _ := setupTaskWithWorktree(t, tasksDir, reposDir, "weird-branch", "repo-weird")
	repoDir := filepath.Join(reposDir, "repo-weird")

	keepCmd := exec.Command("git", "branch", "keepme")
	keepCmd.Dir = repoDir
	if out, err := keepCmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch keepme: %v\n%s", err, out)
	}

	task, err := manifest.Read(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	task.Repos[0].WorktreeBranch = "keepme"
	if err := manifest.Write(task, taskDir); err != nil {
		t.Fatal(err)
	}

	// In --json mode warnings are carried in the success payload, so assert via stdout.
	stdout, _, err := executeCmd(t, "clean", "weird-branch", "--force", "--json")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}

	listCmd := exec.Command("git", "branch", "--list", "keepme")
	listCmd.Dir = repoDir
	listOut, _ := listCmd.Output()
	if len(listOut) == 0 {
		t.Errorf("clean deleted non-ws/ branch 'keepme' that it should have preserved")
	}

	// The clean JSON payload must include a warning identifying why the branch
	// delete was skipped. Silence on a tampered manifest would erase the audit trail.
	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal clean --json: %v\n%s", err, stdout)
	}
	foundWarning := false
	for _, w := range payload.Warnings {
		if strings.Contains(w, "keepme") && strings.Contains(w, "not a ws/ branch") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning about non-ws/ branch skip; got warnings: %v", payload.Warnings)
	}
}

func TestCleanRemovesTaskDir(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	taskDir, _ := setupTaskWithWorktree(t, tasksDir, reposDir, "taskdir-clean", "repo-td")

	_, _, err := executeCmd(t, "clean", "taskdir-clean", "--force")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}

	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("task dir still exists after clean")
	}
}

func TestCleanArchive(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	setupTaskWithWorktree(t, tasksDir, reposDir, "archive-me", "repo-arch")

	_, _, err := executeCmd(t, "clean", "archive-me", "--force", "--archive")
	if err != nil {
		t.Fatalf("clean --archive: %v", err)
	}

	// Task dir should be gone from its original location
	if _, err := os.Stat(filepath.Join(tasksDir, "archive-me")); !os.IsNotExist(err) {
		t.Errorf("task dir should be moved out of its original location")
	}

	// Archived dir should exist under _archive/
	archivedDir := filepath.Join(tasksDir, "_archive", "archive-me")
	if _, err := os.Stat(archivedDir); err != nil {
		t.Errorf("archived dir should exist at %s: %v", archivedDir, err)
	}
}

func TestCleanPartialWorktrees(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	_, worktreeDir := setupTaskWithWorktree(t, tasksDir, reposDir, "partial-clean", "repo-partial")

	// Pre-remove the worktree directory to simulate partial state
	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatal(err)
	}

	// Clean should succeed even though the worktree dir is already gone
	_, _, err := executeCmd(t, "clean", "partial-clean", "--force")
	if err != nil {
		t.Fatalf("clean with partial worktrees: %v", err)
	}

	// Task dir should still be removed
	if _, err := os.Stat(filepath.Join(tasksDir, "partial-clean")); !os.IsNotExist(err) {
		t.Errorf("task dir still exists")
	}
}

func TestCleanDirtyWorktreeRejectsWithoutForce(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	_, worktreeDir := setupTaskWithWorktree(t, tasksDir, reposDir, "dirty-clean", "repo-dirty")

	// Make worktree dirty
	if err := os.WriteFile(filepath.Join(worktreeDir, "uncommitted.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// clean WITHOUT --force should error
	_, stderr, err := executeCmd(t, "clean", "dirty-clean")
	if err == nil {
		t.Error("clean without --force should reject dirty worktree, got nil error")
	}
	if !containsString(stderr, "dirty") && !containsString(stderr, "uncommitted") {
		t.Errorf("error should mention dirty/uncommitted: %s", stderr)
	}

	// Task dir should still exist (clean was rejected)
	if _, err := os.Stat(filepath.Join(tasksDir, "dirty-clean")); os.IsNotExist(err) {
		t.Error("task dir should still exist after rejected clean")
	}
}

func TestCleanNonExistentTask(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	_, _, err := executeCmd(t, "clean", "no-such-task", "--force")
	if err == nil {
		t.Error("clean of non-existent task: expected error")
	}
}

// When stdin is not a TTY (the normal test environment), clean without --force
// must fail with a clear error rather than silently prompting or proceeding.
func TestCleanRequiresForceInNonTTY(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	taskDir, _ := setupTaskWithWorktree(t, tasksDir, reposDir, "needs-force", "repo-needs-force")

	// Test env has no TTY; no --force → expect error
	_, stderr, err := executeCmd(t, "clean", "needs-force")
	if err == nil {
		t.Error("clean without --force in non-TTY mode: expected error, got nil")
	}
	if !containsString(stderr, "force") {
		t.Errorf("error should mention --force: %s", stderr)
	}

	// Task dir must still exist
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Error("task dir was removed even though clean should have rejected")
	}
}

// JSON mode must require --force regardless of TTY state.
func TestCleanJSONRequiresForce(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	setupTaskWithWorktree(t, tasksDir, reposDir, "json-needs-force", "repo-jnf")

	_, stderr, err := executeCmd(t, "clean", "json-needs-force", "--json")
	if err == nil {
		t.Error("clean --json without --force: expected error")
	}
	if !containsString(stderr, "force") {
		t.Errorf("stderr should mention --force: %s", stderr)
	}
}
