package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestJSONOutputConsistency exercises --json on every command that supports it
// and asserts:
//   1. stdout is valid JSON
//   2. stderr is empty on success (no leaking plain-text output)
//
// This locks down the agent-facing contract.
func TestJSONOutputConsistency(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	// Set up a single test task with a worktree so info/status/add-context
	// have something to operate on.
	setupTaskWithWorktree(t, tasksDir, reposDir, "json-check", "repo-json")

	cases := []struct {
		name string
		args []string
	}{
		{"list", []string{"list", "--json"}},
		{"info", []string{"info", "json-check", "--json"}},
		{"status", []string{"status", "json-check", "--json"}},
		{"open", []string{"open", "json-check", "--json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := executeCmd(t, tc.args...)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if stderr != "" {
				t.Errorf("stderr should be empty in --json mode on success, got:\n%s", stderr)
			}
			var v any
			if err := json.Unmarshal([]byte(stdout), &v); err != nil {
				t.Errorf("stdout is not valid JSON: %v\nstdout:\n%s", err, stdout)
			}
		})
	}
}

// TestJSONErrorFormat asserts error output in --json mode uses the
// {"error": "message"} shape on stderr (not stdout), so agents can
// distinguish success payloads from errors.
func TestJSONErrorFormat(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	// Trigger an error: info on a non-existent task
	stdout, stderr, err := executeCmd(t, "info", "no-such-task", "--json")
	if err == nil {
		t.Fatal("expected error for non-existent task, got nil")
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on error in --json mode, got: %s", stdout)
	}

	var errObj map[string]string
	if jerr := json.Unmarshal([]byte(stderr), &errObj); jerr != nil {
		t.Fatalf("stderr is not valid JSON: %v\nstderr: %s", jerr, stderr)
	}
	if _, ok := errObj["error"]; !ok {
		t.Errorf("error JSON missing 'error' key: %v", errObj)
	}
}

// TestJSONNoStdoutLeaks specifically checks that commands with warnings don't
// leak warning text onto stdout in --json mode.
func TestJSONNoStdoutLeaks(t *testing.T) {
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)
	t.Setenv("CS_REPOS_DIR", reposDir)

	// Set up a repo
	repoDir := filepath.Join(reposDir, "repo-leak-test")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initTestRepo(t, repoDir)

	// Create a task with a non-existent context doc to trigger a warning
	stdout, _, err := executeCmd(t, "create", "leak-test",
		"--description", "leak check",
		"--repo", "repo-leak-test",
		"--context-doc", "/nonexistent/fake.txt",
		"--json",
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Stdout must be pure JSON — no warning text should appear there
	var v map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &v); jerr != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", jerr, stdout)
	}

	// warnings field should exist in the JSON payload (not on stderr as text)
	warns, ok := v["warnings"]
	if !ok {
		t.Error("JSON payload missing 'warnings' key")
	} else if _, isSlice := warns.([]any); !isSlice {
		t.Errorf("warnings should be an array, got %T", warns)
	}
}

