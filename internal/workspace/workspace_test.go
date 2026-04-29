package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brian-lai/clean-slate/internal/manifest"
	"github.com/brian-lai/clean-slate/internal/workspace"
)

func TestCreateTaskDir(t *testing.T) {
	tasksDir := t.TempDir()
	taskDir, _, _, err := workspace.Create(tasksDir, "my-task", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(taskDir, "context")); err != nil {
		t.Errorf("context/ dir not created: %v", err)
	}

	want := filepath.Join(tasksDir, "my-task")
	if taskDir != want {
		t.Errorf("taskDir = %q, want %q", taskDir, want)
	}
}

func TestCopyContextDocs(t *testing.T) {
	tasksDir := t.TempDir()

	srcFile := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(srcFile, []byte("hello notes"), 0644); err != nil {
		t.Fatal(err)
	}

	taskDir, copied, warnings, err := workspace.Create(tasksDir, "my-task", []string{srcFile})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(copied) != 1 || copied[0] != filepath.Join("context", "notes.txt") {
		t.Errorf("copied = %v, want [context/notes.txt]", copied)
	}

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
	if err := os.MkdirAll(filepath.Join(tasksDir, "existing-task"), 0755); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := workspace.Create(tasksDir, "existing-task", nil)
	if !errors.Is(err, workspace.ErrTaskExists) {
		t.Errorf("Create existing task = %v, want ErrTaskExists", err)
	}
}

func TestInvalidTaskName(t *testing.T) {
	tasksDir := t.TempDir()
	_, _, _, err := workspace.Create(tasksDir, "invalid name", nil)
	if !errors.Is(err, manifest.ErrInvalidTaskName) {
		t.Errorf("Create invalid name = %v, want ErrInvalidTaskName", err)
	}
}

func TestEmptyContextPaths(t *testing.T) {
	tasksDir := t.TempDir()
	taskDir, copied, warnings, err := workspace.Create(tasksDir, "my-task", []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(copied) > 0 {
		t.Errorf("copied = %v, want empty", copied)
	}

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
	_, copied, warnings, err := workspace.Create(tasksDir, "my-task", []string{"/nonexistent/file.txt"})
	if err != nil {
		t.Fatalf("Create should succeed with warnings: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected warning for non-existent context doc, got none")
	}
	if len(copied) != 0 {
		t.Errorf("copied = %v, want empty (source did not exist)", copied)
	}
}

func TestDuplicateContextDocFilename(t *testing.T) {
	tasksDir := t.TempDir()

	// Two source files with the same basename in different dirs
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	f1 := filepath.Join(dir1, "notes.txt")
	f2 := filepath.Join(dir2, "notes.txt")
	if err := os.WriteFile(f1, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}

	taskDir, copied, warnings, err := workspace.Create(tasksDir, "my-task", []string{f1, f2})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Should have a warning about the collision
	if len(warnings) == 0 {
		t.Error("expected warning for duplicate filename, got none")
	}

	// Both files should be in copied list, with distinct destinations
	if len(copied) != 2 {
		t.Fatalf("copied = %v, want 2 entries", copied)
	}
	if copied[0] == copied[1] {
		t.Errorf("copied entries are identical: %v (should be deduplicated)", copied)
	}

	// Both destination files should exist with their original content
	data1, err := os.ReadFile(filepath.Join(taskDir, copied[0]))
	if err != nil {
		t.Fatalf("read %s: %v", copied[0], err)
	}
	data2, err := os.ReadFile(filepath.Join(taskDir, copied[1]))
	if err != nil {
		t.Fatalf("read %s: %v", copied[1], err)
	}
	if string(data1) == string(data2) {
		t.Errorf("both files have content %q; second source should not have overwritten first", string(data1))
	}
}
