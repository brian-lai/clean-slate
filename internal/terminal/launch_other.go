//go:build !darwin

package terminal

// Launch on non-darwin platforms has no supported terminal scripting target
// (iTerm2 and Terminal.app are macOS-only). It returns the Fallback cd-command
// string so callers can present it to the user for manual execution. The
// error return is always nil — there is no I/O, no subprocess, and no failure
// mode on this path.
func Launch(dir string) (fallback string, err error) {
	return Fallback(dir), nil
}
