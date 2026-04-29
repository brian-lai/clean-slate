package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brian-lai/clean-slate/cmd"
	"github.com/brian-lai/clean-slate/internal/manifest"
)

// writeManifest drops a task.json into tasksDir/<name>/ for list-test fixtures.
func writeManifest(t *testing.T, tasksDir, name, desc string, repoCount int) {
	t.Helper()
	taskDir := filepath.Join(tasksDir, name)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}
	repos := make([]manifest.RepoRef, repoCount)
	for i := 0; i < repoCount; i++ {
		repos[i] = manifest.RepoRef{Name: name + "-repo", Source: "/nowhere"}
	}
	task := manifest.Task{
		Name:        name,
		CreatedAt:   time.Now().UTC(),
		Description: desc,
		Repos:       repos,
		ContextDocs: []string{},
	}
	if err := manifest.Write(task, taskDir); err != nil {
		t.Fatal(err)
	}
}

func TestListMultipleTasks(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "task-alpha", "First task", 1)
	writeManifest(t, tasksDir, "task-beta", "Second task", 2)

	stdout, _, err := executeCmd(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if stdout == "" {
		t.Error("list output is empty")
	}
	// Both tasks should appear in output
	for _, name := range []string{"task-alpha", "task-beta"} {
		if !containsString(stdout, name) {
			t.Errorf("list output missing %q:\n%s", name, stdout)
		}
	}
}

func TestListEmpty(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	stdout, _, err := executeCmd(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Empty list is OK; should mention no tasks or produce no task rows
	if containsString(stdout, "task-") {
		t.Errorf("unexpected task rows in empty list:\n%s", stdout)
	}
}

func TestListJSON(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "task-one", "First", 1)
	writeManifest(t, tasksDir, "task-two", "Second", 0)

	stdout, stderr, err := executeCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty in --json mode, got: %s", stderr)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\nstdout: %s", err, stdout)
	}
	if len(result) != 2 {
		t.Errorf("list --json returned %d entries, want 2", len(result))
	}
}

func TestListJSONEmpty(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	stdout, _, err := executeCmd(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("empty list --json should still be valid JSON: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("empty tasks dir should return empty array, got %d entries", len(result))
	}
}

// containsString is a tiny string-contains helper to avoid pulling in strings
// where a readable test predicate is clearer.
func containsString(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// avoid unused import warning until cmd.* are referenced in tests that grow
var _ = cmd.ExecuteArgs
