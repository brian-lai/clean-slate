package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blai/clean-slate/internal/manifest"
	"github.com/blai/clean-slate/internal/workspace"
)

func TestCreateTaskDir(t *testing.T) {
	tasksDir := t.TempDir()
	taskDir, _, err := workspace.Create(tasksDir, "my-task", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Task dir should exist
	if _, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir not created: %v", err)
	}

	// context/ subdir should exist
	if _, err := os.Stat(filepath.Join(taskDir, "context")); err != nil {
		t.Errorf("context/ dir not created: %v", err)
	}

	// Task dir should be <tasksDir>/<taskName>
	want := filepath.Join(tasksDir, "my-task")
	if taskDir != want {
		t.Errorf("taskDir = %q, want %q", taskDir, want)
	}
}

func TestCopyContextDocs(t *testing.T) {
	tasksDir := t.TempDir()

	// Create a source file
	srcFile := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(srcFile, []byte("hello notes"), 0644); err != nil {
		t.Fatal(err)
	}

	taskDir, warnings, err := workspace.Create(tasksDir, "my-task", []string{srcFile})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// File should be copied into context/
	destFile := filepath.Join(taskDir, "context", "notes.txt")
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("context doc not copied: %v", err)
	}
	if string(data) != "hello notes" {
		t.Errorf("file content = %q, want %q", string(data), "hello notes")
	}
}

func TestAlreadyExists(t *testing.T) {
	tasksDir := t.TempDir()
	// Create the task dir first
	if err := os.MkdirAll(filepath.Join(tasksDir, "existing-task"), 0755); err != nil {
		t.Fatal(err)
	}

	_, _, err := workspace.Create(tasksDir, "existing-task", nil)
	if !errors.Is(err, manifest.ErrTaskExists) {
		t.Errorf("Create existing task = %v, want ErrTaskExists", err)
	}
}

func TestInvalidTaskName(t *testing.T) {
	tasksDir := t.TempDir()
	_, _, err := workspace.Create(tasksDir, "invalid name", nil)
	if !errors.Is(err, manifest.ErrInvalidTaskName) {
		t.Errorf("Create invalid name = %v, want ErrInvalidTaskName", err)
	}
}

func TestEmptyContextPaths(t *testing.T) {
	tasksDir := t.TempDir()
	taskDir, warnings, err := workspace.Create(tasksDir, "my-task", []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// context/ dir should exist but be empty
	entries, err := os.ReadDir(filepath.Join(taskDir, "context"))
	if err != nil {
		t.Fatalf("read context dir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("context/ has %d files, want 0", len(entries))
	}
}

func TestNonExistentContextDoc(t *testing.T) {
	tasksDir := t.TempDir()
	_, warnings, err := workspace.Create(tasksDir, "my-task", []string{"/nonexistent/file.txt"})
	if err != nil {
		t.Fatalf("Create should succeed with warnings: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected warning for non-existent context doc, got none")
	}
}
