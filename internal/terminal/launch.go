//go:build darwin

package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// Launch opens a new terminal window (iTerm2 if running, else Terminal.app)
// cd'd into dir. Returns a fallback string (to print) if neither terminal
// is available.
//
// The path is passed through osascript's argv rather than interpolated into
// the AppleScript source, so there is no parser injection vector.
func Launch(dir string) (fallback string, err error) {
	if isRunning("iTerm2") {
		return "", runOSAScriptWithArg(ITermScriptTemplate(), dir)
	}
	if hasApp("Terminal") {
		return "", runOSAScriptWithArg(TerminalAppScriptTemplate(), dir)
	}
	return Fallback(dir), nil
}

// ITermScriptTemplate returns AppleScript that reads the target directory
// from argv (item 1) and opens a new iTerm2 tab cd'd into it. The path is
// never interpolated into the script source: osascript passes it as argv,
// and AppleScript's `quoted form of` handles shell quoting for `do shell script`.
func ITermScriptTemplate() string {
	return `on run argv
    set dir to item 1 of argv
    tell application "iTerm"
        activate
        tell current window
            create tab with default profile
            tell current session to write text ("cd " & quoted form of dir)
        end tell
    end tell
end run`
}

// TerminalAppScriptTemplate is the Terminal.app equivalent of ITermScriptTemplate.
func TerminalAppScriptTemplate() string {
	return `on run argv
    set dir to item 1 of argv
    tell application "Terminal"
        activate
        do script ("cd " & quoted form of dir)
    end tell
end run`
}

// ITermScript is kept for backward-compatibility with existing tests.
// It returns the template; callers that want to actually launch a terminal
// should use Launch instead.
func ITermScript(dir string) string {
	return ITermScriptTemplate()
}

// TerminalAppScript — same note as ITermScript.
func TerminalAppScript(dir string) string {
	return TerminalAppScriptTemplate()
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
// Terminal.app is always present on macOS, so we short-circuit.
func hasApp(appName string) bool {
	if appName == "Terminal" {
		return true
	}
	_, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application "Finder" to exists application file id "com.apple.%s"`, strings.ToLower(appName)),
	).Output()
	return err == nil
}

// runOSAScriptWithArg runs an AppleScript template and passes arg as argv[1].
// The argument is never parsed as AppleScript source.
func runOSAScriptWithArg(script, arg string) error {
	cmd := exec.Command("osascript", "-e", script, arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w\n%s", err, out)
	}
	return nil
}
