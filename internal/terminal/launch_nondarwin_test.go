//go:build !darwin

package terminal_test

import (
	"testing"

	"github.com/brian-lai/clean-slate/internal/terminal"
)

// On non-darwin platforms, Launch has no supported terminal to drive
// (iTerm2/Terminal.app are macOS-only). It must degrade gracefully by
// returning the Fallback cd-command string so the caller can present it
// to the user for manual execution. Returning an error here would break
// `cs open` on Linux and defeat the point of cross-platform distribution.
func TestLaunchReturnsFallbackOnNonDarwin(t *testing.T) {
	const dir = "/tmp/foo"

	fallback, err := terminal.Launch(dir)
	if err != nil {
		t.Fatalf("Launch on non-darwin should not error, got: %v", err)
	}

	want := terminal.Fallback(dir)
	if fallback != want {
		t.Errorf("Launch on non-darwin: got %q, want %q", fallback, want)
	}
}
