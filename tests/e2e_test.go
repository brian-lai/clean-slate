// Package tests contains end-to-end acceptance tests that invoke the compiled
// cs binary. These tests build the binary once per run and set CS_TASKS_DIR
// and CS_REPOS_DIR to temp dirs so they don't touch the user's real workspace.
//
// Run: go test ./tests/
// Skip with -short: go test -short ./...
package tests_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// binaryPath is set once, lazily, to the path of the built cs binary.
var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
)

// buildBinary builds cs into a temp dir and returns the path. Idempotent
// across test invocations via sync.Once.
func buildBinary(t *testing.T) string {
	t.Helper()
	binaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cs-e2e-")
		if err != nil {
			binaryErr = err
			return
		}
		path := filepath.Join(dir, "cs")
		// Build from the repo root (two levels up from tests/).
		cmd := exec.Command("go", "build", "-o", path, ".")
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			binaryErr = fmt.Errorf("build cs: %w\n%s", err, out)
			return
		}
		binaryPath = path
	})
	if binaryErr != nil {
		t.Fatalf("binary setup: %v", binaryErr)
	}
	return binaryPath
}

// initRepo creates a real git repo with a single commit on main.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "e2e@test.com")
	run("config", "user.name", "E2E")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

// envForTest returns the env slice with CS_TASKS_DIR/CS_REPOS_DIR set.
func envForTest(tasksDir, reposDir string) []string {
	env := os.Environ()
	env = append(env, "CS_TASKS_DIR="+tasksDir, "CS_REPOS_DIR="+reposDir)
	return env
}

// runBinary invokes the compiled binary with args; returns stdout/stderr/exit.
func runBinary(t *testing.T, tasksDir, reposDir string, args ...string) (string, string, int) {
	t.Helper()
	bin := buildBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = envForTest(tasksDir, reposDir)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else {
			t.Fatalf("run cs: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exit
}

func TestE2ECreateHeadless(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "e2e-repo"))

	stdout, stderr, exit := runBinary(t, tasksDir, reposDir,
		"create", "e2e-task",
		"--description", "e2e create test",
		"--repo", "e2e-repo",
		"--json",
	)
	if exit != 0 {
		t.Fatalf("create exit=%d stderr=%s", exit, stderr)
	}

	// Parse JSON output
	var result struct {
		Task struct {
			Name string `json:"name"`
		} `json:"task"`
		TaskDir string `json:"task_dir"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if result.Task.Name != "e2e-task" {
		t.Errorf("task.name = %q, want %q", result.Task.Name, "e2e-task")
	}

	// Task dir must exist on disk
	if _, err := os.Stat(result.TaskDir); err != nil {
		t.Errorf("task_dir %s does not exist: %v", result.TaskDir, err)
	}
}

func TestE2EList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "listrepo"))

	// Create two tasks
	for _, name := range []string{"task-one", "task-two"} {
		_, stderr, exit := runBinary(t, tasksDir, reposDir,
			"create", name,
			"--description", "list test",
			"--repo", "listrepo",
			"--json",
		)
		if exit != 0 {
			t.Fatalf("create %s: exit=%d stderr=%s", name, exit, stderr)
		}
	}

	stdout, _, exit := runBinary(t, tasksDir, reposDir, "list", "--json")
	if exit != 0 {
		t.Fatalf("list exit=%d", exit)
	}

	var list []map[string]any
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("list JSON: %v\n%s", err, stdout)
	}
	if len(list) != 2 {
		t.Errorf("list returned %d entries, want 2", len(list))
	}
}

func TestE2EInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "inforepo"))

	_, _, exit := runBinary(t, tasksDir, reposDir,
		"create", "info-task",
		"--description", "info test",
		"--repo", "inforepo",
		"--json",
	)
	if exit != 0 {
		t.Fatal("create failed")
	}

	stdout, _, exit := runBinary(t, tasksDir, reposDir, "info", "info-task", "--json")
	if exit != 0 {
		t.Fatalf("info exit=%d", exit)
	}

	var task struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(stdout), &task); err != nil {
		t.Fatalf("info JSON: %v\n%s", err, stdout)
	}
	if task.Name != "info-task" {
		t.Errorf("info name = %q, want %q", task.Name, "info-task")
	}
}

func TestE2EStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "statusrepo"))

	_, _, exit := runBinary(t, tasksDir, reposDir,
		"create", "status-task",
		"--description", "status test",
		"--repo", "statusrepo",
		"--json",
	)
	if exit != 0 {
		t.Fatal("create failed")
	}

	stdout, _, exit := runBinary(t, tasksDir, reposDir, "status", "status-task")
	if exit != 0 {
		t.Fatalf("status exit=%d", exit)
	}
	if !strings.Contains(stdout, "statusrepo") {
		t.Errorf("status output missing repo name:\n%s", stdout)
	}
}

func TestE2EAddContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "ctxrepo"))

	_, _, exit := runBinary(t, tasksDir, reposDir,
		"create", "ctx-task",
		"--description", "ctx test",
		"--repo", "ctxrepo",
		"--json",
	)
	if exit != 0 {
		t.Fatal("create failed")
	}

	doc := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(doc, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, exit := runBinary(t, tasksDir, reposDir, "add-context", "ctx-task", doc, "--json")
	if exit != 0 {
		t.Fatalf("add-context exit=%d stderr=%s", exit, stderr)
	}

	// Verify the file landed in context/
	copied := filepath.Join(tasksDir, "ctx-task", "context", "note.txt")
	data, err := os.ReadFile(copied)
	if err != nil {
		t.Fatalf("file not copied: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content mismatch: %q", string(data))
	}
}

func TestE2EClean(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "cleanrepo"))

	_, _, exit := runBinary(t, tasksDir, reposDir,
		"create", "clean-task",
		"--description", "clean test",
		"--repo", "cleanrepo",
		"--json",
	)
	if exit != 0 {
		t.Fatal("create failed")
	}

	_, stderr, exit := runBinary(t, tasksDir, reposDir, "clean", "clean-task", "--force", "--json")
	if exit != 0 {
		t.Fatalf("clean exit=%d stderr=%s", exit, stderr)
	}

	// Task dir must be gone
	if _, err := os.Stat(filepath.Join(tasksDir, "clean-task")); !os.IsNotExist(err) {
		t.Error("task dir should be removed after clean --force")
	}
}

func TestE2ECreateAlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initRepo(t, filepath.Join(reposDir, "duprepo"))

	args := []string{"create", "dup-task",
		"--description", "dup test",
		"--repo", "duprepo",
		"--json",
	}
	_, _, exit := runBinary(t, tasksDir, reposDir, args...)
	if exit != 0 {
		t.Fatal("first create failed")
	}

	// Second create with same name should exit non-zero
	stdout, stderr, exit := runBinary(t, tasksDir, reposDir, args...)
	if exit == 0 {
		t.Error("second create should exit non-zero")
	}

	// Error should be on stderr as JSON, stdout empty
	if stdout != "" {
		t.Errorf("stdout should be empty on error, got: %s", stdout)
	}
	var errObj map[string]string
	if err := json.Unmarshal([]byte(stderr), &errObj); err != nil {
		t.Errorf("stderr not valid JSON: %v\n%s", err, stderr)
	}
	if _, ok := errObj["error"]; !ok {
		t.Errorf("error JSON missing 'error' key: %v", errObj)
	}
}

func TestE2EVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	stdout, _, exit := runBinary(t, tasksDir, reposDir, "--version")
	if exit != 0 {
		t.Fatalf("--version exit=%d", exit)
	}
	if !strings.Contains(stdout, "cs version") {
		t.Errorf("--version output unexpected: %s", stdout)
	}
}
