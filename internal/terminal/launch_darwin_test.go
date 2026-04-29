//go:build darwin

package terminal_test

import (
	"strings"
	"testing"

	"github.com/brian-lai/clean-slate/internal/terminal"
)

// Templates are argv-based, so the path should NOT appear in the script source.
// Injection safety comes from osascript passing the argument separately.

func TestITermScriptTemplateUsesArgv(t *testing.T) {
	script := terminal.ITermScriptTemplate()
	if !strings.Contains(script, "argv") {
		t.Errorf("iTerm template should use argv, got:\n%s", script)
	}
	// Path should NOT be in the template — it's passed via argv at run time.
	if strings.Contains(script, "/tmp") {
		t.Errorf("template should not contain any path literal: %s", script)
	}
}

func TestTerminalAppScriptTemplateUsesArgv(t *testing.T) {
	script := terminal.TerminalAppScriptTemplate()
	if !strings.Contains(script, "argv") {
		t.Errorf("Terminal.app template should use argv, got:\n%s", script)
	}
	if strings.Contains(script, "/tmp") {
		t.Errorf("template should not contain any path literal: %s", script)
	}
}

func TestITermScriptBackwardCompat(t *testing.T) {
	// ITermScript(dir) retained for backward compat: returns the template.
	// The path is ignored in the script; it's passed as argv at run time.
	script := terminal.ITermScript("/tmp/my-task")
	if strings.Contains(strings.ToLower(script), "iterm") == false {
		t.Errorf("iTerm script should mention iTerm: %s", script)
	}
}

func TestTerminalAppScriptBackwardCompat(t *testing.T) {
	script := terminal.TerminalAppScript("/tmp/my-task")
	if strings.Contains(strings.ToLower(script), "terminal") == false {
		t.Errorf("Terminal.app script should mention Terminal: %s", script)
	}
}
