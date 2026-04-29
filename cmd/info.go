package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brian-lai/clean-slate/internal/config"
	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show details for a task workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
	taskName := args[0]

	cfg := config.Load()
	taskDir := filepath.Join(cfg.TasksDir, taskName)

	if _, err := os.Stat(taskDir); err != nil {
		if os.IsNotExist(err) {
			werr := fmt.Errorf("task %q not found at %s", taskName, taskDir)
			outputError(cmd, useJSON, werr)
			return werr
		}
		outputError(cmd, useJSON, err)
		return err
	}

	task, err := manifest.Read(taskDir)
	if err != nil {
		outputError(cmd, useJSON, fmt.Errorf("read manifest for %q: %w", taskName, err))
		return err
	}

	if useJSON {
		return outputJSON(cmd, task)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Task:        %s\n", task.Name)
	fmt.Fprintf(out, "Directory:   %s\n", taskDir)
	fmt.Fprintf(out, "Created:     %s\n", task.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	if task.JiraTicket != "" {
		fmt.Fprintf(out, "JIRA:        %s\n", task.JiraTicket)
	}
	fmt.Fprintf(out, "Description: %s\n", task.Description)

	if len(task.Repos) > 0 {
		fmt.Fprintln(out, "\nRepositories:")
		for _, r := range task.Repos {
			fmt.Fprintf(out, "  %s\n", r.Name)
			fmt.Fprintf(out, "    source:   %s\n", r.Source)
			fmt.Fprintf(out, "    worktree: %s\n", filepath.Join(taskDir, r.WorktreePath))
			fmt.Fprintf(out, "    branch:   %s (from %s)\n", r.WorktreeBranch, r.BaseBranch)
		}
	}

	if len(task.ContextDocs) > 0 {
		fmt.Fprintln(out, "\nContext Docs:")
		for _, d := range task.ContextDocs {
			fmt.Fprintf(out, "  %s\n", d)
		}
	}

	return nil
}
