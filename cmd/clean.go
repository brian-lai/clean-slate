package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blai/clean-slate/internal/config"
	"github.com/blai/clean-slate/internal/git"
	"github.com/blai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

var (
	cleanForce   bool
	cleanArchive bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean <name>",
	Short: "Tear down a task workspace (remove worktrees and task directory)",
	Args:  cobra.ExactArgs(1),
	RunE:  runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Skip confirmation and force-remove dirty worktrees")
	cleanCmd.Flags().BoolVar(&cleanArchive, "archive", false, "Move task dir to _archive/ instead of deleting")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
	taskName := args[0]

	cfg := config.Load()
	taskDir := filepath.Join(cfg.TasksDir, taskName)

	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		werr := fmt.Errorf("task %q not found at %s", taskName, taskDir)
		outputError(cmd, useJSON, werr)
		return werr
	}

	task, err := manifest.Read(taskDir)
	if err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	// If not forced, check for dirty worktrees first
	if !cleanForce {
		var dirty []string
		for _, r := range task.Repos {
			wt := filepath.Join(taskDir, r.WorktreePath)
			if _, err := os.Stat(wt); err == nil {
				if isDirty(wt) {
					dirty = append(dirty, r.Name)
				}
			}
		}
		if len(dirty) > 0 {
			werr := fmt.Errorf("worktrees have uncommitted changes: %s (use --force to override)", strings.Join(dirty, ", "))
			outputError(cmd, useJSON, werr)
			return werr
		}
	}

	// In JSON mode without --force, we require --force since no interactive confirmation is possible
	if useJSON && !cleanForce {
		werr := fmt.Errorf("--json mode requires --force (no interactive confirmation)")
		outputError(cmd, useJSON, werr)
		return werr
	}

	// Remove each worktree; collect warnings but keep going on individual failures
	var warnings []string
	for _, r := range task.Repos {
		wt := filepath.Join(taskDir, r.WorktreePath)
		if _, err := os.Stat(wt); os.IsNotExist(err) {
			// Already gone; skip silently
			continue
		}
		if err := git.RemoveWorktree(wt); err != nil {
			warnings = append(warnings, fmt.Sprintf("remove worktree %s: %v", r.Name, err))
		}
	}

	// Prune worktree metadata in each source repo (cleans up any stale entries)
	for _, r := range task.Repos {
		pruneWorktrees(r.Source)
	}

	// Either archive or delete the task dir
	if cleanArchive {
		archiveDir := filepath.Join(cfg.TasksDir, "_archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			outputError(cmd, useJSON, fmt.Errorf("create archive dir: %w", err))
			return err
		}
		dest := filepath.Join(archiveDir, taskName)
		if err := os.Rename(taskDir, dest); err != nil {
			outputError(cmd, useJSON, fmt.Errorf("archive task: %w", err))
			return err
		}
	} else {
		if err := os.RemoveAll(taskDir); err != nil {
			outputError(cmd, useJSON, fmt.Errorf("remove task dir: %w", err))
			return err
		}
	}

	if useJSON {
		result := map[string]any{
			"task":     taskName,
			"archived": cleanArchive,
			"warnings": warnings,
		}
		if warnings == nil {
			result["warnings"] = []string{}
		}
		return outputJSON(cmd, result)
	}

	action := "removed"
	if cleanArchive {
		action = fmt.Sprintf("archived to %s", filepath.Join(cfg.TasksDir, "_archive", taskName))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Task %q %s\n", taskName, action)
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", w)
	}
	return nil
}

// isDirty returns true if the worktree has uncommitted changes.
func isDirty(worktreePath string) bool {
	out, err := gitOutput(worktreePath, "status", "--porcelain")
	if err != nil {
		// If git status fails, assume clean (caller can decide)
		return false
	}
	return strings.TrimSpace(out) != ""
}

// pruneWorktrees runs 'git worktree prune' in the source repo, ignoring errors.
func pruneWorktrees(repoPath string) {
	_, _ = gitOutput(repoPath, "worktree", "prune")
}
