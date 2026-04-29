package manifest_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blai/clean-slate/internal/manifest"
)

func TestTaskRoundTrip(t *testing.T) {
	task := manifest.Task{
		Name:        "my-task",
		CreatedAt:   time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		JiraTicket:  "RNA-549",
		Description: "Test task",
		Repos: []manifest.RepoRef{
			{
				Name:           "rna",
				Source:         "/projects/repos/rna",
				WorktreePath:   "rna",
				WorktreeBranch: "ws/my-task",
				BaseBranch:     "main",
			},
		},
		ContextDocs: []string{"context/notes.txt"},
	}

	dir := t.TempDir()
	if err := manifest.Write(task, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := manifest.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Name != task.Name {
		t.Errorf("Name = %q, want %q", got.Name, task.Name)
	}
	if got.JiraTicket != task.JiraTicket {
		t.Errorf("JiraTicket = %q, want %q", got.JiraTicket, task.JiraTicket)
	}
	if len(got.ContextDocs) != 1 || got.ContextDocs[0] != "context/notes.txt" {
		t.Errorf("ContextDocs = %v, want [context/notes.txt]", got.ContextDocs)
	}
}

func TestWriteAndRead(t *testing.T) {
	task := manifest.Task{
		Name:        "test-task",
		CreatedAt:   time.Now().UTC(),
		Description: "A test task",
		Repos:       []manifest.RepoRef{},
		ContextDocs: []string{},
	}

	dir := t.TempDir()
	if err := manifest.Write(task, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "task.json")); err != nil {
		t.Errorf("task.json not created: %v", err)
	}

	got, err := manifest.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != task.Name {
		t.Errorf("Name = %q, want %q", got.Name, task.Name)
	}
	if got.Description != task.Description {
		t.Errorf("Description = %q, want %q", got.Description, task.Description)
	}
}

func TestValidateMissingName(t *testing.T) {
	task := manifest.Task{Description: "some desc"}
	err := manifest.Validate(task)
	if !errors.Is(err, manifest.ErrInvalidTaskName) {
		t.Errorf("Validate empty name = %v, want ErrInvalidTaskName", err)
	}
}

func TestValidateMissingDescription(t *testing.T) {
	task := manifest.Task{Name: "valid-name"}
	err := manifest.Validate(task)
	if err == nil {
		t.Error("Validate missing description: expected error, got nil")
	}
}

func TestValidateInvalidTaskNameChars(t *testing.T) {
	invalid := []string{"my task", "feat:thing", "a..b", "a~b", "a^b", "a:b", "a b"}
	for _, name := range invalid {
		task := manifest.Task{Name: name, Description: "desc"}
		err := manifest.Validate(task)
		if !errors.Is(err, manifest.ErrInvalidTaskName) {
			t.Errorf("Validate name %q = %v, want ErrInvalidTaskName", name, err)
		}
	}

	valid := []string{"my-task", "task_1", "RNA.549", "a", "my-task-2"}
	for _, name := range valid {
		task := manifest.Task{Name: name, Description: "desc"}
		err := manifest.Validate(task)
		if err != nil {
			t.Errorf("Validate valid name %q = %v, want nil", name, err)
		}
	}
}

func TestValidateEmptyRepoName(t *testing.T) {
	task := manifest.Task{
		Name:        "valid",
		Description: "desc",
		Repos: []manifest.RepoRef{
			{Name: "", Source: "/some/path"},
		},
	}
	err := manifest.Validate(task)
	if err == nil {
		t.Error("Validate empty repo name: expected error, got nil")
	}
}

func TestReadNonExistent(t *testing.T) {
	_, err := manifest.Read("/nonexistent/path/that/does/not/exist")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Read nonexistent = %v, want os.ErrNotExist", err)
	}
}
