package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/brian-lai/clean-slate/internal/cslock"
	"github.com/brian-lai/clean-slate/internal/version"
	"github.com/spf13/cobra"
)

// errReported marks errors that a RunE handler has already emitted to
// stdout/stderr via outputError. The top-level Execute seam uses errors.Is
// to skip re-reporting those — so Cobra-produced errors (wrong arg count,
// unknown flags) still get formatted, while per-command errors don't double-print.
var errReported = errors.New("cs: error already reported")

var jsonOutput bool
var outWriter io.Writer = os.Stdout
var errWriter io.Writer = os.Stderr

var rootCmd = &cobra.Command{
	Use:           "cs",
	Short:         "Clean Slate — task workspace manager",
	Long:          `cs creates and manages isolated task workspaces under ~/projects/tasks/.`,
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true, // commands emit their own error output (plain or JSON)
}

func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	reportIfNeeded(err)
	os.Exit(1)
}

// ExecuteArgs is used in tests to invoke the CLI with specific args.
func ExecuteArgs(args []string) error {
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outWriter)
	rootCmd.SetErr(errWriter)
	err := rootCmd.Execute()
	if err != nil {
		reportIfNeeded(err)
	}
	return err
}

// reportIfNeeded emits an error that no RunE has already handled. Errors from
// Cobra itself (arg-count, unknown flag, unknown subcommand) reach this seam
// unwrapped because SilenceErrors=true on rootCmd stops Cobra from printing
// them — we format them ourselves so the JSON contract holds in --json mode.
func reportIfNeeded(err error) {
	if errors.Is(err, errReported) {
		return
	}
	useJSON, _ := rootCmd.PersistentFlags().GetBool("json")
	outputError(rootCmd, useJSON, err)
}

// SetOutput redirects stdout/stderr for testing.
func SetOutput(out, err io.Writer) {
	outWriter = out
	errWriter = err
	rootCmd.SetOut(out)
	rootCmd.SetErr(err)
}

// ResetOutput restores stdout/stderr to os defaults and resets flag state for test isolation.
func ResetOutput() {
	outWriter = os.Stdout
	errWriter = os.Stderr
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(nil)
	// Reset persistent flags
	rootCmd.PersistentFlags().Set("json", "false") //nolint
	// Reset create command flags
	createCmd.Flags().Set("jira", "")        //nolint
	createCmd.Flags().Set("description", "") //nolint
	createCmd.Flags().Set("repo", "")        //nolint (reset slice)
	createCmd.Flags().Set("context-doc", "") //nolint
	createRepos = nil
	createContextDoc = nil
	// Reset status flags
	statusCmd.Flags().Set("verbose", "false") //nolint
	statusVerbose = false
	// Reset clean flags
	cleanCmd.Flags().Set("force", "false")   //nolint
	cleanCmd.Flags().Set("archive", "false") //nolint
	cleanForce = false
	cleanArchive = false
}

// --- JSON output contract ---
//
// Commands that support --json MUST adhere to the following contract:
//   1. On success: indented JSON on stdout via outputJSON; stderr empty.
//   2. On error: JSON error object {"error": "..."} on stderr via outputError;
//      stdout empty.
//   3. Warnings must appear inside the success JSON payload (typically under
//      a "warnings" field), NOT as plain text on stderr.
//   4. Empty collections must serialize as [] not null (initialize slices
//      before marshaling).
//
// Tests in cmd/json_test.go enforce these invariants across all commands.

// outputJSON writes v as indented JSON to the command's stdout writer.
// Used for success payloads in --json mode.
func outputJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputError writes the error to the command's stderr writer.
// In JSON mode: a JSON {"error": "..."} object. Otherwise: plain "Error: ...".
// The returned value wraps err with errReported so reportIfNeeded at the
// Execute seam skips it, avoiding double-printing.
func outputError(cmd *cobra.Command, useJSON bool, err error) error {
	if useJSON {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(cmd.ErrOrStderr(), string(data))
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err.Error())
	}
	return fmt.Errorf("%w: %w", errReported, err)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

// lockTask acquires the per-task advisory lock at
// <tasksDir>/.cs-locks/<taskname>.lock. On contention it returns a
// CLI-friendly error shaped for v0.2.0: "task %q is locked by PID %d
// (started %s ago)" — or "an unknown process" when the race-with-writer
// payload window yielded no PID. Callers defer lock.Release().
func lockTask(tasksDir, taskName string) (*cslock.Lock, error) {
	lockPath := filepath.Join(tasksDir, ".cs-locks", taskName+".lock")
	lock, err := cslock.Acquire(lockPath)
	if err != nil {
		var locked *cslock.ErrLocked
		if errors.As(err, &locked) {
			if locked.Info.PID == -1 {
				return nil, fmt.Errorf("task %q is locked by an unknown process", taskName)
			}
			return nil, fmt.Errorf("task %q is locked by PID %d (started %s ago)",
				taskName, locked.Info.PID, time.Since(locked.Info.Started).Truncate(time.Second))
		}
		return nil, err
	}
	return lock, nil
}
