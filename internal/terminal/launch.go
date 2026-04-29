package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// Launch opens a new terminal window (iTerm2 if running, else Terminal.app)
// cd'd into dir. Returns a fallback string (to print) if neither terminal
// is available.
func Launch(dir string) (fallback string, err error) {
	if isRunning("iTerm2") {
		return "", runOSAScript(ITermScript(dir))
	}
	if hasApp("Terminal") {
		return "", runOSAScript(TerminalAppScript(dir))
	}
	return Fallback(dir), nil
}

// ITermScript returns AppleScript that opens a new iTerm2 tab cd'd into dir.
// The path is single-quote-escaped using the POSIX '\” idiom so that paths
// containing single quotes don't break the shell command iTerm runs.
func ITermScript(dir string) string {
	safe := escapeSingleQuotes(dir)
	return fmt.Sprintf(`tell application "iTerm"
    activate
    tell current window
        create tab with default profile
        tell current session to write text "cd '%s'"
    end tell
end tell`, safe)
}

// TerminalAppScript returns AppleScript that opens a new Terminal.app window
// cd'd into dir.
func TerminalAppScript(dir string) string {
	safe := escapeSingleQuotes(dir)
	return fmt.Sprintf(`tell application "Terminal"
    activate
    do script "cd '%s'"
end tell`, safe)
}

// Fallback returns a human-readable message containing the path so the user
// can cd manually when no supported terminal is available.
func Fallback(dir string) string {
	return fmt.Sprintf("cd %s", dir)
}

// escapeSingleQuotes replaces ' with '\” so that the resulting string can be
// safely placed inside single quotes in a POSIX shell command.
func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, `'`, `'\''`)
}

// isRunning checks whether an application is currently running (macOS).
func isRunning(appName string) bool {
	out, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "System Events" to (name of processes) contains "%s"`, appName),
	).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// hasApp checks whether an application is installed (macOS).
func hasApp(appName string) bool {
	_, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "Finder" to exists application file id "com.apple.%s"`, strings.ToLower(appName)),
	).Output()
	// Fallback to assuming Terminal.app is always present on macOS
	if appName == "Terminal" {
		return true
	}
	return err == nil
}

// runOSAScript executes an AppleScript via osascript.
func runOSAScript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w\n%s", err, out)
	}
	return nil
}
