package terminal_test

import (
	"strings"
	"testing"

	"github.com/brian-lai/clean-slate/internal/terminal"
)

// Fallback is cross-platform — lives in launch_shared.go and is exercised on
// every OS. Darwin-only tests (AppleScript templates, backward-compat wrappers)
// live in launch_darwin_test.go.

func TestFallbackShellQuotesPath(t *testing.T) {
	// Fallback output is meant to be copy-pasted into a shell. The path must
	// be single-quoted so spaces, $, and other shell metacharacters don't
	// break the command.
	out := terminal.Fallback("/tmp/my task")
	if !strings.Contains(out, "'/tmp/my task'") {
		t.Errorf("Fallback should single-quote path with spaces: %s", out)
	}

	// Paths with embedded single quotes must use the POSIX '\'' escape.
	out = terminal.Fallback(`/tmp/o'reilly`)
	if !strings.Contains(out, `'/tmp/o'\''reilly'`) {
		t.Errorf("Fallback should escape single quotes via '\\'': %s", out)
	}
}

func TestFallbackContainsPath(t *testing.T) {
	out := terminal.Fallback("/tmp/my-task")
	if !strings.Contains(out, "/tmp/my-task") {
		t.Errorf("Fallback output should contain path: %s", out)
	}
}
