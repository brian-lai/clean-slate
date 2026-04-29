package cmd_test

import (
	"encoding/json"
	"testing"
)

func TestInfoExistingTask(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "my-info-task", "Info test description", 2)

	stdout, _, err := executeCmd(t, "info", "my-info-task")
	if err != nil {
		t.Fatalf("info: %v", err)
	}

	// Output should mention the name and description
	for _, want := range []string{"my-info-task", "Info test description"} {
		if !containsString(stdout, want) {
			t.Errorf("info output missing %q:\n%s", want, stdout)
		}
	}
}

func TestInfoNonExistent(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	_, _, err := executeCmd(t, "info", "does-not-exist")
	if err == nil {
		t.Error("info for non-existent task: expected error, got nil")
	}
}

func TestInfoJSON(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "json-info", "JSON info test", 1)

	stdout, stderr, err := executeCmd(t, "info", "json-info", "--json")
	if err != nil {
		t.Fatalf("info --json: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty in --json mode, got: %s", stderr)
	}

	var task struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(stdout), &task); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if task.Name != "json-info" {
		t.Errorf("name = %q, want %q", task.Name, "json-info")
	}
	if task.Description != "JSON info test" {
		t.Errorf("description = %q, want %q", task.Description, "JSON info test")
	}
}
