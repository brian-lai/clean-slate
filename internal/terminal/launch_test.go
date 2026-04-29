package terminal_test

import (
	"strings"
	"testing"

	"github.com/blai/clean-slate/internal/terminal"
)

func TestITermScript(t *testing.T) {
	script := terminal.ITermScript("/tmp/my-task")
	if !strings.Contains(script, "/tmp/my-task") {
		t.Errorf("iTerm script missing path: %s", script)
	}
	if !strings.Contains(strings.ToLower(script), "iterm") {
		t.Errorf("iTerm script should mention iTerm: %s", script)
	}
}

func TestTerminalAppScript(t *testing.T) {
	script := terminal.TerminalAppScript("/tmp/my-task")
	if !strings.Contains(script, "/tmp/my-task") {
		t.Errorf("Terminal.app script missing path: %s", script)
	}
	if !strings.Contains(strings.ToLower(script), "terminal") {
		t.Errorf("Terminal.app script should mention Terminal: %s", script)
	}
}

func TestITermScriptEscapesQuotes(t *testing.T) {
	// Path with a single quote should be escaped (or rejected)
	script := terminal.ITermScript(`/tmp/my'task`)
	// Path should not appear raw in a way that breaks the AppleScript syntax
	if strings.Contains(script, `/tmp/my'task`) && !strings.Contains(script, `\'`) && !strings.Contains(script, `'\''`) {
		// If the path appears unescaped and without escaping, that's a shell-injection risk
		t.Errorf("iTerm script does not escape single quote in path: %s", script)
	}
}

func TestFallbackOutput(t *testing.T) {
	// Fallback returns the path so the user can cd themselves
	out := terminal.Fallback("/tmp/my-task")
	if !strings.Contains(out, "/tmp/my-task") {
		t.Errorf("Fallback output should contain path: %s", out)
	}
}
