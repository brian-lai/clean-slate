package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brian-lai/clean-slate/internal/config"
	"github.com/brian-lai/clean-slate/internal/git"
	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/brian-lai/clean-slate/internal/tui"
	"github.com/brian-lai/clean-slate/internal/workspace"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

type createResult struct {
	Task     manifest.Task `json:"task"`
	TaskDir  string        `json:"task_dir"`
	Warnings []string      `json:"warnings"`
}

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new task workspace",
	Long:  `Create a new task workspace. If name and --description are not provided and stdin is a terminal, runs interactive prompts.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
}

var (
	createJira       string
	createDesc       string
	createRepos      []string
	createContextDoc []string
)

func init() {
	createCmd.Flags().StringVar(&createJira, "jira", "", "JIRA ticket (e.g. PROJ-549)")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Brief description of the task (required)")
	createCmd.Flags().StringArrayVar(&createRepos, "repo", nil, "Repository name from ~/projects/repos/ (repeatable)")
	createCmd.Flags().StringArrayVar(&createContextDoc, "context-doc", nil, "Path to a supporting document (repeatable)")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")

	// Collect inputs from args + flags
	inputs := tui.CreateInputs{
		Description: createDesc,
		JiraTicket:  createJira,
		Repos:       createRepos,
		ContextDocs: createContextDoc,
	}
	if len(args) > 0 {
		inputs.Name = args[0]
	}

	// If required fields are missing, decide whether to prompt interactively or error out.
	needed := tui.NeedsPrompt(inputs)
	if len(needed) > 0 {
		if useJSON || !isatty.IsTerminal(os.Stdin.Fd()) {
			// Headless/piped mode: cannot prompt, must error.
			err := fmt.Errorf("missing required input(s): %v (provide via flags or run in a TTY for interactive prompts)", needed)
			return outputError(cmd, useJSON, err)
		}

		cfg := config.Load()
		repoNames, _ := git.ListRepos(cfg.ReposDir) // error is non-fatal; empty list is fine
		filled, err := tui.PromptCreate(repoNames, inputs)
		if err != nil {
			return outputError(cmd, useJSON, err)
		}
		inputs = filled
	}

	taskName := inputs.Name
	createDesc = inputs.Description
	createJira = inputs.JiraTicket
	createRepos = inputs.Repos
	createContextDoc = inputs.ContextDocs

	// Pre-validate inputs that can be checked without side effects.
	if err := manifest.ValidateName(taskName); err != nil {
		return outputError(cmd, useJSON, err)
	}
	if createDesc == "" {
		err := manifest.ErrDescriptionRequired
		return outputError(cmd, useJSON, err)
	}

	cfg := config.Load()

	// Create workspace directory and copy context docs.
	// copiedDocs is the list of docs that actually landed on disk (relative paths).
	taskDir, copiedDocs, warnings, err := workspace.Create(cfg.TasksDir, taskName, createContextDoc)
	if err != nil {
		return outputError(cmd, useJSON, err)
	}

	// Track created worktrees so we can roll back on failure.
	var addedWorktrees []string
	rollback := func() {
		for _, wt := range addedWorktrees {
			_ = git.RemoveWorktree(wt)
		}
		_ = os.RemoveAll(taskDir)
	}

	repos := []manifest.RepoRef{}
	for _, repoName := range createRepos {
		repoPath := filepath.Join(cfg.ReposDir, repoName)
		worktreeDest := filepath.Join(taskDir, repoName)
		branchName := "ws/" + taskName

		baseBranch, wtErr := git.AddWorktree(repoPath, worktreeDest, branchName)
		if wtErr != nil {
			warnings = append(warnings, fmt.Sprintf("skip repo %s: %v", repoName, wtErr))
			continue
		}
		addedWorktrees = append(addedWorktrees, worktreeDest)

		repos = append(repos, manifest.RepoRef{
			Name:           repoName,
			Source:         repoPath,
			WorktreePath:   repoName,
			WorktreeBranch: branchName,
			BaseBranch:     baseBranch,
		})
	}

	// Build final manifest using only docs that were actually copied.
	if copiedDocs == nil {
		copiedDocs = []string{}
	}
	task := manifest.Task{
		Name:        taskName,
		CreatedAt:   time.Now().UTC(),
		JiraTicket:  createJira,
		Description: createDesc,
		Repos:       repos,
		ContextDocs: copiedDocs,
	}
	if err := manifest.Validate(task); err != nil {
		rollback()
		return outputError(cmd, useJSON, err)
	}
	if err := manifest.Write(task, taskDir); err != nil {
		rollback()
		return outputError(cmd, useJSON, err)
	}

	if warnings == nil {
		warnings = []string{}
	}
	result := createResult{Task: task, TaskDir: taskDir, Warnings: warnings}

	if useJSON {
		return outputJSON(cmd, result)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created task %q at %s\n", taskName, taskDir)
	if len(repos) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Worktrees:\n")
		for _, r := range repos {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s → %s (branch: %s)\n", r.Name, filepath.Join(taskDir, r.Name), r.WorktreeBranch)
		}
	}
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", w)
	}

	return nil
}
