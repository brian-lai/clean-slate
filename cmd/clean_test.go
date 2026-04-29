package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
