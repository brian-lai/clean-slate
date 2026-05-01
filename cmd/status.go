package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/brian-lai/clean-slate/internal/config"
	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

type repoStatus struct {
	Name           string `json:"name"`
	WorktreePath   string `json:"worktree_path"`
	Branch         string `json:"branch"`
	Clean          bool   `json:"clean"`
	Dirty          bool   `json:"dirty"`
	UncommittedN   int    `json:"uncommitted_files"`
	LastCommitHash string `json:"last_commit_hash,omitempty"`
	LastCommitMsg  string `json:"last_commit_message,omitempty"`
	AheadBehind    string `json:"ahead_behind,omitempty"`
	Missing        bool   `json:"missing,omitempty"`
	Error          string `json:"error,omitempty"`
}

type statusResult struct {
	Task  string       `json:"task"`
	Repos []repoStatus `json:"repos"`
}

var statusVerbose bool

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show git status for each worktree in a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "Show last commit, ahead/behind, and uncommitted count")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
	taskName := args[0]

	cfg := config.Load()
	taskDir := filepath.Join(cfg.TasksDir, taskName)

	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		werr := fmt.Errorf("task %q not found at %s", taskName, taskDir)
		return outputError(cmd, useJSON, werr)
	}

	task, err := manifest.Read(taskDir)
	if err != nil {
		return outputError(cmd, useJSON, err)
	}

	var repos []repoStatus
	for _, r := range task.Repos {
		worktreePath := filepath.Join(taskDir, r.WorktreePath)
		repos = append(repos, collectRepoStatus(r.Name, worktreePath, statusVerbose))
	}

	result := statusResult{Task: task.Name, Repos: repos}

	if useJSON {
		return outputJSON(cmd, result)
	}

	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if statusVerbose {
		fmt.Fprintln(tw, "REPO\tBRANCH\tSTATUS\tUNCOMMITTED\tLAST COMMIT\tMESSAGE")
		for _, rs := range repos {
			if rs.Missing {
				fmt.Fprintf(tw, "%s\t-\tmissing\t-\t-\t%s\n", rs.Name, rs.Error)
				continue
			}
			state := "clean"
			if rs.Dirty {
				state = "dirty"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
				rs.Name, rs.Branch, state, rs.UncommittedN,
				rs.LastCommitHash, truncate(rs.LastCommitMsg, 50),
			)
		}
	} else {
		fmt.Fprintln(tw, "REPO\tBRANCH\tSTATUS")
		for _, rs := range repos {
			if rs.Missing {
				fmt.Fprintf(tw, "%s\t-\tmissing (%s)\n", rs.Name, rs.Error)
				continue
			}
			state := "clean"
			if rs.Dirty {
				state = fmt.Sprintf("dirty (%d uncommitted)", rs.UncommittedN)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", rs.Name, rs.Branch, state)
		}
	}
	return tw.Flush()
}

// collectRepoStatus runs git commands in the worktree to collect status info.
// Failures are captured in the result rather than propagated, so one broken
// worktree doesn't mask the others.
func collectRepoStatus(name, worktreePath string, verbose bool) repoStatus {
	rs := repoStatus{Name: name, WorktreePath: worktreePath}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		rs.Missing = true
		rs.Error = "worktree directory not found"
		return rs
	}

	// Current branch
	if out, err := gitOutput(worktreePath, "branch", "--show-current"); err == nil {
		rs.Branch = strings.TrimSpace(out)
	}

	// Dirty / uncommitted count
	if out, err := gitOutput(worktreePath, "status", "--porcelain"); err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if out == "" {
			rs.Clean = true
			rs.UncommittedN = 0
		} else {
			rs.Dirty = true
			rs.UncommittedN = len(lines)
		}
	}

	if verbose {
		if out, err := gitOutput(worktreePath, "log", "-1", "--format=%h"); err == nil {
			rs.LastCommitHash = strings.TrimSpace(out)
		}
		if out, err := gitOutput(worktreePath, "log", "-1", "--format=%s"); err == nil {
			rs.LastCommitMsg = strings.TrimSpace(out)
		}
		// Ahead/behind vs upstream
		if out, err := gitOutput(worktreePath, "rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
			rs.AheadBehind = strings.TrimSpace(out)
		}
	}

	return rs
}

// gitOutput runs a git command in dir and returns trimmed stdout or an error.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, stderr.String())
	}
	return string(out), nil
}
