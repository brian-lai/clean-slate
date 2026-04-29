package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/blai/clean-slate/internal/manifest"
)

// setupTaskWithWorktree initializes a repo under reposDir/<repoName>, creates a task
// at tasksDir/<taskName>, adds a worktree of that repo inside the task, and writes
// the manifest. Returns the task dir and the worktree dir.
func setupTaskWithWorktree(t *testing.T, tasksDir, reposDir, taskName, repoName string) (string, string) {
	t.Helper()

	// Init the source repo
	repoDir := filepath.Join(reposDir, repoName)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initTestRepo(t, repoDir)

	// Create the task dir
	taskDir := filepath.Join(tasksDir, taskName)
	if err := os.MkdirAll(filepath.Join(taskDir, "context"), 0755); err != nil {
		t.Fatal(err)
	}

	// Add the worktree
	worktreeDest := filepath.Join(taskDir, repoName)
	branchName := "ws/" + taskName

	cmd := exec.Command("git", "worktree", "add", worktreeDest, "-b", branchName, "main")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}

	// Write manifest
	task := manifest.Task{
		Name:        taskName,
		CreatedAt:   time.Now().UTC(),
		Description: "status test",
		Repos: []manifest.RepoRef{{
			Name:           repoName,
			Source:         repoDir,
			WorktreePath:   repoName,
			WorktreeBranch: branchName,
			BaseBranch:     "main",
		}},
		ContextDocs: []string{},
	}
	if err := manifest.Write(task, taskDir); err != nil {
		t.Fatal(err)
	}

	return taskDir, worktreeDest
}

func TestStatusCleanWorktree(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	setupTaskWithWorktree(t, tasksDir, reposDir, "clean-status", "repo-clean")

	stdout, _, err := executeCmd(t, "status", "clean-status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !containsString(stdout, "repo-clean") {
		t.Errorf("status output missing repo name:\n%s", stdout)
	}
	if !containsString(stdout, "clean") {
		t.Errorf("status output should indicate 'clean':\n%s", stdout)
	}
}

func TestStatusDirtyWorktree(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	_, worktreeDir := setupTaskWithWorktree(t, tasksDir, reposDir, "dirty-status", "repo-dirty")

	// Create an uncommitted file in the worktree
	if err := os.WriteFile(filepath.Join(worktreeDir, "new-file.txt"), []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeCmd(t, "status", "dirty-status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !containsString(stdout, "dirty") {
		t.Errorf("status output should indicate 'dirty':\n%s", stdout)
	}
}

func TestStatusVerbose(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	setupTaskWithWorktree(t, tasksDir, reposDir, "verbose-status", "repo-verbose")

	stdout, _, err := executeCmd(t, "status", "verbose-status", "--verbose")
	if err != nil {
		t.Fatalf("status --verbose: %v", err)
	}
	// Verbose output should include the last commit message ("init") and branch name
	if !containsString(stdout, "init") {
		t.Errorf("verbose status missing commit message:\n%s", stdout)
	}
	if !containsString(stdout, "ws/verbose-status") {
		t.Errorf("verbose status missing branch name:\n%s", stdout)
	}
}

func TestStatusMissingWorktree(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	_, worktreeDir := setupTaskWithWorktree(t, tasksDir, reposDir, "missing-status", "repo-missing")

	// Remove the worktree directory (but not the manifest)
	if err := os.RemoveAll(worktreeDir); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeCmd(t, "status", "missing-status")
	if err != nil {
		t.Fatalf("status should not crash on missing worktree: %v", err)
	}
	if !containsString(stdout, "missing") && !containsString(stdout, "not found") {
		t.Errorf("status should warn about missing worktree:\n%s", stdout)
	}
}

func TestStatusNonExistentTask(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	_, _, err := executeCmd(t, "status", "does-not-exist")
	if err == nil {
		t.Error("status for non-existent task: expected error, got nil")
	}
}
