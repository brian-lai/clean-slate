package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/brian-lai/clean-slate/internal/config"
	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

type listEntry struct {
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	JiraTicket  string    `json:"jira_ticket,omitempty"`
	Description string    `json:"description"`
	RepoCount   int       `json:"repo_count"`
	TaskDir     string    `json:"task_dir"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all task workspaces",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, _ []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")

	cfg := config.Load()
	entries, err := collectTasks(cfg.TasksDir)
	if err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	// Always sort by CreatedAt descending (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	if useJSON {
		// Ensure non-null even when empty
		if entries == nil {
			entries = []listEntry{}
		}
		return outputJSON(cmd, entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
		return nil
	}

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tCREATED\tREPOS\tDESCRIPTION")
	for _, e := range entries {
		desc := truncate(e.Description, 60)
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n",
			e.Name,
			e.CreatedAt.Local().Format("2006-01-02 15:04"),
			e.RepoCount,
			desc,
		)
	}
	return tw.Flush()
}

// collectTasks scans tasksDir for subdirectories containing task.json.
func collectTasks(tasksDir string) ([]listEntry, error) {
	dirEntries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []listEntry{}, nil
		}
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}

	var entries []listEntry
	for _, d := range dirEntries {
		if !d.IsDir() {
			continue
		}
		taskDir := filepath.Join(tasksDir, d.Name())
		task, err := manifest.Read(taskDir)
		if err != nil {
			// Skip directories without a task.json (could be _archive/ or junk)
			continue
		}
		entries = append(entries, listEntry{
			Name:        task.Name,
			CreatedAt:   task.CreatedAt,
			JiraTicket:  task.JiraTicket,
			Description: task.Description,
			RepoCount:   len(task.Repos),
			TaskDir:     taskDir,
		})
	}
	return entries, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
