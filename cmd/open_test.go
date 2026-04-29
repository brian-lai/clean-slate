package cmd_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestOpenNonExistentTask(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	_, _, err := executeCmd(t, "open", "no-such-task")
	if err == nil {
		t.Error("open on non-existent task: expected error, got nil")
	}
}

// In --json mode, open just returns the path — it does NOT launch a terminal,
// which would be a footgun in agent contexts.
func TestOpenJSONReturnsPath(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	writeManifest(t, tasksDir, "open-json", "open test", 0)

	stdout, stderr, err := executeCmd(t, "open", "open-json", "--json")
	if err != nil {
		t.Fatalf("open --json: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty in --json mode, got: %s", stderr)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	wantDir := filepath.Join(tasksDir, "open-json")
	if result["task_dir"] != wantDir {
		t.Errorf("task_dir = %q, want %q", result["task_dir"], wantDir)
	}
}
