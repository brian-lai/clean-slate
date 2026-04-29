package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blai/clean-slate/internal/config"
	"github.com/blai/clean-slate/internal/terminal"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <name>",
	Short: "Open a new terminal window in a task workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
	taskName := args[0]

	cfg := config.Load()
	taskDir := filepath.Join(cfg.TasksDir, taskName)

	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		werr := fmt.Errorf("task %q not found at %s", taskName, taskDir)
		outputError(cmd, useJSON, werr)
		return werr
	}

	if useJSON {
		// In JSON mode, just return the path — launching a terminal from an
		// agent context is rarely what's wanted.
		return outputJSON(cmd, map[string]string{"task_dir": taskDir})
	}

	fallback, err := terminal.Launch(taskDir)
	if err != nil {
		outputError(cmd, useJSON, err)
		return err
	}
	if fallback != "" {
		fmt.Fprintln(cmd.OutOrStdout(), fallback)
	}
	return nil
}
