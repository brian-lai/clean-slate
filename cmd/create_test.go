package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/brian-lai/clean-slate/cmd"
	"github.com/brian-lai/clean-slate/internal/manifest"
)

// initTestRepo creates a minimal git repo with an initial commit.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
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
}

// setupTestEnv creates temp tasks and repos dirs, inits the given repos, and
// sets the CS_TASKS_DIR / CS_REPOS_DIR env vars.
func setupTestEnv(t *testing.T, repoNames []string) (tasksDir, reposDir string) {
	t.Helper()
	tasksDir = t.TempDir()
	reposDir = t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	for _, name := range repoNames {
		dir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		initTestRepo(t, dir)
	}
	return
}

func executeCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOutput(&outBuf, &errBuf)
	defer cmd.ResetOutput()
	err = cmd.ExecuteArgs(args)
	return outBuf.String(), errBuf.String(), err
}

func TestCreateCommandHeadless(t *testing.T) {
	tasksDir, _ := setupTestEnv(t, []string{"test-repo"})

	_, _, err := executeCmd(t, "create", "my-task",
		"--description", "A test task",
		"--repo", "test-repo",
	)
	if err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	// Task dir should exist
	taskDir := filepath.Join(tasksDir, "my-task")
	if _, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir not created: %v", err)
	}

	// Manifest should be valid
	task, err := manifest.Read(taskDir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if task.Name != "my-task" {
		t.Errorf("task.Name = %q, want %q", task.Name, "my-task")
	}
	if task.Description != "A test task" {
		t.Errorf("task.Description = %q, want %q", task.Description, "A test task")
	}
	if len(task.Repos) != 1 || task.Repos[0].Name != "test-repo" {
		t.Errorf("task.Repos = %v, want [{test-repo ...}]", task.Repos)
	}
}

func TestCreateCommandJSON(t *testing.T) {
	tasksDir, _ := setupTestEnv(t, []string{"test-repo"})

	stdout, stderr, err := executeCmd(t, "create", "json-task",
		"--description", "JSON test",
		"--repo", "test-repo",
		"--json",
	)
	if err != nil {
		t.Fatalf("create --json failed: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty in --json mode with no warnings, got: %s", stderr)
	}

	var result struct {
		Task struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Repos       []struct {
				Name string `json:"name"`
			} `json:"repos"`
			ContextDocs []string `json:"context_docs"`
		} `json:"task"`
		TaskDir  string   `json:"task_dir"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if result.Task.Name != "json-task" {
		t.Errorf("task.name = %q, want %q", result.Task.Name, "json-task")
	}
	if result.Task.Description != "JSON test" {
		t.Errorf("task.description = %q, want %q", result.Task.Description, "JSON test")
	}
	if len(result.Task.Repos) != 1 || result.Task.Repos[0].Name != "test-repo" {
		t.Errorf("task.repos = %v, want [test-repo]", result.Task.Repos)
	}
	// context_docs must be [] not null for agent-friendly output
	if result.Task.ContextDocs == nil {
		t.Error("task.context_docs is null; want [] (empty array) for agent-friendly output")
	}
	// warnings must be [] not null
	if result.Warnings == nil {
		t.Error("warnings is null; want []")
	}
	// task_dir should point to the actual on-disk location
	if result.TaskDir != filepath.Join(tasksDir, "json-task") {
		t.Errorf("task_dir = %q, want %q", result.TaskDir, filepath.Join(tasksDir, "json-task"))
	}
	if _, err := os.Stat(result.TaskDir); err != nil {
		t.Errorf("task_dir does not exist on disk: %v", err)
	}
}

func TestCreateCommandMissingDescription(t *testing.T) {
	setupTestEnv(t, []string{})

	_, _, err := executeCmd(t, "create", "my-task")
	if err == nil {
		t.Error("expected error for missing description, got nil")
	}
}

// TestCreateCommandNoSideEffectsOnValidationFailure verifies that when
// pre-validation fails (e.g., invalid task name), no task directory is created.
func TestCreateCommandNoSideEffectsOnValidationFailure(t *testing.T) {
	tasksDir, _ := setupTestEnv(t, []string{})

	_, _, err := executeCmd(t, "create", "invalid name",
		"--description", "test",
	)
	if err == nil {
		t.Fatal("expected error for invalid task name, got nil")
	}

	// Task dir should NOT exist — validation failed before workspace.Create ran
	taskDir := filepath.Join(tasksDir, "invalid name")
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Errorf("task dir %q should not exist after validation failure, stat err=%v", taskDir, err)
	}
}

// TestCreateCommandContextDocsInManifestMatchDisk verifies that the manifest
// only records context docs that were actually copied. If a context doc source
// doesn't exist, it's skipped with a warning and NOT added to the manifest.
func TestCreateCommandContextDocsInManifestMatchDisk(t *testing.T) {
	tasksDir, _ := setupTestEnv(t, []string{"test-repo"})

	// Create one real doc and reference one non-existent doc
	realDoc := filepath.Join(t.TempDir(), "real.txt")
	if err := os.WriteFile(realDoc, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeCmd(t, "create", "doc-test",
		"--description", "doc test",
		"--repo", "test-repo",
		"--context-doc", realDoc,
		"--context-doc", "/nonexistent/fake.txt",
	)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	taskDir := filepath.Join(tasksDir, "doc-test")
	task, err := manifest.Read(taskDir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	// Only the real doc should appear in context_docs
	if len(task.ContextDocs) != 1 {
		t.Fatalf("ContextDocs = %v, want exactly 1 entry (the real doc)", task.ContextDocs)
	}
	if task.ContextDocs[0] != "context/real.txt" {
		t.Errorf("ContextDocs[0] = %q, want %q", task.ContextDocs[0], "context/real.txt")
	}

	// Verify the file actually exists on disk
	if _, err := os.Stat(filepath.Join(taskDir, task.ContextDocs[0])); err != nil {
		t.Errorf("manifest references %q but file does not exist: %v", task.ContextDocs[0], err)
	}
}
