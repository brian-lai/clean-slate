package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestArgErrorHumanMode verifies that Cobra's arg-count errors surface to
// stderr as a human-readable message, not silent exit 1. Before v0.1.4,
// SilenceErrors + SilenceUsage on the root command swallowed these entirely.
func TestArgErrorHumanMode(t *testing.T) {
	// `open` accepts exactly 1 arg; passing 2 should be rejected.
	stdout, stderr, err := executeCmd(t, "open", "one", "two")
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on arg error, got: %q", stdout)
	}
	if stderr == "" {
		t.Fatal("stderr is empty — arg-error is silent (the v0.1.3 bug)")
	}
	if !strings.Contains(stderr, "accepts") && !strings.Contains(stderr, "arg") {
		t.Errorf("stderr should describe the arg problem, got: %q", stderr)
	}
}

// TestArgErrorJSONMode verifies that in --json mode the arg-error is also
// visible, as a JSON {"error": "..."} object on stderr, preserving the JSON
// output contract documented in cmd/root.go.
func TestArgErrorJSONMode(t *testing.T) {
	stdout, stderr, err := executeCmd(t, "--json", "open", "one", "two")
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on arg error, got: %q", stdout)
	}
	if stderr == "" {
		t.Fatal("stderr is empty — arg-error is silent (the v0.1.3 bug)")
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
		t.Fatalf("stderr should be valid JSON in --json mode: %v\nstderr: %q", err, stderr)
	}
	if payload["error"] == "" {
		t.Errorf("JSON payload missing 'error' field: %q", stderr)
	}
}
