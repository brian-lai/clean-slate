package cmd_test

import (
	"strings"
	"testing"
)

func TestCompletionZsh(t *testing.T) {
	stdout, _, err := executeCmd(t, "completion", "zsh")
	if err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	// Cobra's zsh completion has a #compdef marker at the top.
	if !strings.Contains(stdout, "#compdef") && !strings.Contains(stdout, "compdef cs") {
		t.Errorf("zsh completion output missing #compdef marker:\n%s", stdout[:min(300, len(stdout))])
	}
}

func TestCompletionBash(t *testing.T) {
	stdout, _, err := executeCmd(t, "completion", "bash")
	if err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	// Cobra's bash completion defines a completion function.
	if !strings.Contains(stdout, "__cs_") && !strings.Contains(stdout, "complete") {
		t.Errorf("bash completion output missing expected markers:\n%s", stdout[:min(300, len(stdout))])
	}
}

func TestCompletionFish(t *testing.T) {
	stdout, _, err := executeCmd(t, "completion", "fish")
	if err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	if !strings.Contains(stdout, "complete -c cs") {
		t.Errorf("fish completion output missing 'complete -c cs':\n%s", stdout[:min(300, len(stdout))])
	}
}

func TestCompletionUnknownShell(t *testing.T) {
	_, _, err := executeCmd(t, "completion", "powershell-but-not-supported")
	if err == nil {
		t.Error("completion with unknown shell: expected error, got nil")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
