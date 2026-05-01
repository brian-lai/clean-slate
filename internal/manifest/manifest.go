package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/brian-lai/clean-slate/internal/atomicio"
)

var (
	ErrInvalidTaskName     = errors.New("invalid task name")
	ErrDescriptionRequired = errors.New("description is required")
	ErrRepoNameRequired    = errors.New("repo name is required")
	ErrRepoSourceRequired  = errors.New("repo source is required")
)

var validTaskName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var invalidTaskNamePatterns = regexp.MustCompile(`\.\.`)

// ValidateName checks that a task name is syntactically valid.
// Rules: must match [a-zA-Z0-9][a-zA-Z0-9._-]*, no ".." sequence.
// Exposed so other packages (workspace) can validate without duplicating the regex.
func ValidateName(name string) error {
	if name == "" || !validTaskName.MatchString(name) || invalidTaskNamePatterns.MatchString(name) {
		return fmt.Errorf("%w: %q (must start with alphanumeric, match [a-zA-Z0-9._-]+, no ..)", ErrInvalidTaskName, name)
	}
	return nil
}

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
	return atomicio.WriteFile(filepath.Join(dir, "task.json"), data, 0644)
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
	if err := ValidateName(task.Name); err != nil {
		return err
	}
	if task.Description == "" {
		return ErrDescriptionRequired
	}
	for i, repo := range task.Repos {
		if repo.Name == "" {
			return fmt.Errorf("repo[%d]: %w", i, ErrRepoNameRequired)
		}
		if repo.Source == "" {
			return fmt.Errorf("repo[%d]: %w", i, ErrRepoSourceRequired)
		}
	}
	return nil
}
