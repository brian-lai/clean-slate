package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	ErrTaskExists            = errors.New("task already exists")
	ErrWorktreeBranchExists  = errors.New("worktree branch already exists")
	ErrNoDefaultBranch       = errors.New("could not detect default branch")
	ErrInvalidTaskName       = errors.New("invalid task name")
)

var validTaskName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var invalidTaskNamePatterns = regexp.MustCompile(`\.\.`)

type Task struct {
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	JiraTicket  string    `json:"jira_ticket,omitempty"`
	Description string    `json:"description"`
	Repos       []RepoRef `json:"repos"`
	ContextDocs []string  `json:"context_docs"` // relative to task dir
}

type RepoRef struct {
	Name           string `json:"name"`
	Source         string `json:"source"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	BaseBranch     string `json:"base_branch"`
}

func Write(task Task, dir string) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "task.json"), data, 0644)
}

func Read(dir string) (Task, error) {
	data, err := os.ReadFile(filepath.Join(dir, "task.json"))
	if err != nil {
		return Task{}, fmt.Errorf("read task.json: %w", err)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return Task{}, fmt.Errorf("parse task.json: %w", err)
	}
	return task, nil
}

func Validate(task Task) error {
	if task.Name == "" || !validTaskName.MatchString(task.Name) || invalidTaskNamePatterns.MatchString(task.Name) {
		return fmt.Errorf("%w: %q (must match [a-zA-Z0-9._-]+, no ..)", ErrInvalidTaskName, task.Name)
	}
	if task.Description == "" {
		return errors.New("description is required")
	}
	for i, repo := range task.Repos {
		if repo.Name == "" {
			return fmt.Errorf("repo[%d].name is empty", i)
		}
		if repo.Source == "" {
			return fmt.Errorf("repo[%d].source is empty", i)
		}
	}
	return nil
}
