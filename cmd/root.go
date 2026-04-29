package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/blai/clean-slate/internal/version"
	"github.com/spf13/cobra"
)

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
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ExecuteArgs is used in tests to invoke the CLI with specific args.
func ExecuteArgs(args []string) error {
	rootCmd.SetArgs(args)
	rootCmd.SetOut(outWriter)
	rootCmd.SetErr(errWriter)
	return rootCmd.Execute()
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
func outputError(cmd *cobra.Command, useJSON bool, err error) {
	if useJSON {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(cmd.ErrOrStderr(), string(data))
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err.Error())
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}
