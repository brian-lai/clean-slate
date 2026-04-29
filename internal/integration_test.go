package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/blai/clean-slate/internal/config"
	"github.com/blai/clean-slate/internal/git"
	"github.com/blai/clean-slate/internal/manifest"
	"github.com/blai/clean-slate/internal/workspace"
)

// initTestRepo creates a git repo with an initial commit at reposDir/name.
func initTestRepo(t *testing.T, reposDir, name string) string {
	t.Helper()
	dir := filepath.Join(reposDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestCreateOrchestrationEndToEnd(t *testing.T) {
	// Set up temp dirs for tasks and repos
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	// Init 2 test git repos
	initTestRepo(t, reposDir, "repo-a")
	initTestRepo(t, reposDir, "repo-b")

	// Create a context doc
	contextDoc := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(contextDoc, []byte("some notes"), 0644); err != nil {
		t.Fatal(err)
	}

	// --- Orchestration ---
	cfg := config.Load()

	taskName := "test-integration"
	repoNames := []string{"repo-a", "repo-b"}
	contextPaths := []string{contextDoc}

	// Step 1: Create workspace directory
	taskDir, copiedDocs, warnings, err := workspace.Create(cfg.TasksDir, taskName, contextPaths)
	if err != nil {
		t.Fatalf("workspace.Create: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(copiedDocs) != 1 || copiedDocs[0] != "context/notes.txt" {
		t.Errorf("copiedDocs = %v, want [context/notes.txt]", copiedDocs)
	}

	// Step 2: Add worktrees for each repo
	var repos []manifest.RepoRef
	for _, repoName := range repoNames {
		repoPath := filepath.Join(cfg.ReposDir, repoName)
		worktreeDest := filepath.Join(taskDir, repoName)
		branchName := "ws/" + taskName

		baseBranch, err := git.AddWorktree(repoPath, worktreeDest, branchName)
		if err != nil {
			t.Fatalf("git.AddWorktree %s: %v", repoName, err)
		}

		repos = append(repos, manifest.RepoRef{
			Name:           repoName,
			Source:         repoPath,
			WorktreePath:   repoName, // relative to task dir
			WorktreeBranch: branchName,
			BaseBranch:     baseBranch,
		})
	}

	// Step 3: Write manifest (use copiedDocs from workspace.Create, not hardcoded)
	task := manifest.Task{
		Name:        taskName,
		CreatedAt:   time.Now().UTC(),
		Description: "Integration test task",
		Repos:       repos,
		ContextDocs: copiedDocs,
	}
	if err := manifest.Validate(task); err != nil {
		t.Fatalf("manifest.Validate: %v", err)
	}
	if err := manifest.Write(task, taskDir); err != nil {
		t.Fatalf("manifest.Write: %v", err)
	}

	// --- Assertions ---

	// Task dir and context/ exist
	if _, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(taskDir, "context")); err != nil {
		t.Errorf("context/ dir missing: %v", err)
	}

	// Context doc copied
	if _, err := os.Stat(filepath.Join(taskDir, "context", "notes.txt")); err != nil {
		t.Errorf("context doc not copied: %v", err)
	}

	// Worktrees exist and are on the right branch
	for _, repoName := range repoNames {
		worktreeDir := filepath.Join(taskDir, repoName)
		if _, err := os.Stat(worktreeDir); err != nil {
			t.Errorf("worktree %s missing: %v", repoName, err)
		}

		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = worktreeDir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git branch in %s: %v", repoName, err)
		}
		branch := string(out[:len(out)-1])
		want := "ws/" + taskName
		if branch != want {
			t.Errorf("worktree %s branch = %q, want %q", repoName, branch, want)
		}
	}

	// Manifest is valid and complete
	saved, err := manifest.Read(taskDir)
	if err != nil {
		t.Fatalf("manifest.Read: %v", err)
	}
	if saved.Name != taskName {
		t.Errorf("manifest.Name = %q, want %q", saved.Name, taskName)
	}
	if len(saved.Repos) != 2 {
		t.Errorf("manifest.Repos len = %d, want 2", len(saved.Repos))
	}
	if len(saved.ContextDocs) != 1 || saved.ContextDocs[0] != "context/notes.txt" {
		t.Errorf("manifest.ContextDocs = %v, want [context/notes.txt]", saved.ContextDocs)
	}
	// Verify context_docs are relative (not absolute)
	if filepath.IsAbs(saved.ContextDocs[0]) {
		t.Errorf("context_docs[0] = %q is absolute, want relative", saved.ContextDocs[0])
	}
}
