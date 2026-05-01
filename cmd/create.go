package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brian-lai/clean-slate/internal/config"
	"github.com/brian-lai/clean-slate/internal/git"
	"github.com/brian-lai/clean-slate/internal/journal"
	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/brian-lai/clean-slate/internal/tui"
	"github.com/brian-lai/clean-slate/internal/workspace"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// createAfterJournalHook is a test-only seam. When non-nil, runCreate invokes
// it immediately after the first journal.Write so tests can deterministically
// observe mid-operation filesystem state. Do NOT use this in production code.
var createAfterJournalHook func()

// SetCreateAfterJournalHook installs a test hook that fires inside runCreate
// right after the first journal.Write. Pass nil to clear.
func SetCreateAfterJournalHook(fn func()) { createAfterJournalHook = fn }

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

	// Acquire the per-task advisory lock BEFORE workspace.Create. Lock lives
	// at <tasksDir>/.cs-locks/<taskname>.lock — outside the task dir so its
	// lifecycle is independent of workspace.Create's existence check and a
	// losing racer never creates an orphan task dir.
	lock, err := lockTask(cfg.TasksDir, taskName)
	if err != nil {
		return outputError(cmd, useJSON, err)
	}
	defer lock.Release()

	// Create workspace directory and copy context docs.
	// copiedDocs is the list of docs that actually landed on disk (relative paths).
	taskDir, copiedDocs, warnings, err := workspace.Create(cfg.TasksDir, taskName, createContextDoc)
	if err != nil {
		return outputError(cmd, useJSON, err)
	}

	// Track created worktrees so we can roll back on failure. The journal
	// below captures the same list to disk so Phase 2's orphan sweep can
	// reverse the operation even after this process is killed.
	var addedWorktrees []string
	journalEntry := journal.Entry{
		Op:        "create",
		PID:       os.Getpid(),
		Started:   time.Now().UTC(),
		TaskDir:   taskDir,
		Worktrees: []string{},
		Branches:  []journal.BranchRef{},
	}

	rollback := func() {
		for _, wt := range addedWorktrees {
			_ = git.RemoveWorktree(wt)
		}
		_ = os.RemoveAll(taskDir)
		_ = journal.Clear(taskDir, journalEntry.PID)
	}

	// Crash-gap note: between workspace.Create above and this first
	// journal.Write, a SIGKILL leaves `taskDir/context/` on disk with NO
	// journal file. Phase 2's ScanOrphans globs `.cs-journal.*` per task dir
	// and therefore cannot recover such an orphan. The window is microseconds
	// and Phase 2's per-task lock narrows it further (the orphan is still
	// discoverable by its lack of task.json when the user runs cs list, but
	// automatic recovery isn't provided). Accepted for v0.2.0; a future
	// hardening could write a "staging" journal at <tasksDir>/.cs-staging/
	// BEFORE workspace.Create to close this gap completely.
	if err := journal.Write(taskDir, journalEntry); err != nil {
		rollback()
		return outputError(cmd, useJSON, err)
	}
	if createAfterJournalHook != nil {
		createAfterJournalHook()
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

		// Re-journal after each successful worktree add so the orphan sweep
		// sees the current set of side effects.
		journalEntry.Worktrees = append(journalEntry.Worktrees, worktreeDest)
		journalEntry.Branches = append(journalEntry.Branches, journal.BranchRef{
			RepoPath: repoPath,
			Branch:   branchName,
		})
		if jErr := journal.Write(taskDir, journalEntry); jErr != nil {
			// A failing journal update post-side-effect is a real risk to
			// durability — roll back rather than persist an unrecoverable state.
			rollback()
			return outputError(cmd, useJSON, fmt.Errorf("update journal: %w", jErr))
		}

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

	// Task is legitimate; clear the journal. A failing Clear is recoverable
	// (Phase 2 sweep will tidy up) — log to warnings but don't fail create.
	if err := journal.Clear(taskDir, journalEntry.PID); err != nil {
		warnings = append(warnings, fmt.Sprintf("clear journal: %v", err))
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
