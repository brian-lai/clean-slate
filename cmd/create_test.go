package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blai/clean-slate/cmd"
	"github.com/blai/clean-slate/internal/manifest"
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
	setupTestEnv(t, []string{"test-repo"})

	stdout, _, err := executeCmd(t, "create", "json-task",
		"--description", "JSON test",
		"--repo", "test-repo",
		"--json",
	)
	if err != nil {
		t.Fatalf("create --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Errorf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if _, ok := result["name"]; !ok {
		t.Errorf("JSON output missing 'name' field: %s", stdout)
	}
}

func TestCreateCommandMissingDescription(t *testing.T) {
	setupTestEnv(t, []string{})

	_, _, err := executeCmd(t, "create", "my-task")
	if err == nil {
		t.Error("expected error for missing description, got nil")
	}
}
