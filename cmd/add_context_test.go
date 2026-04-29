package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brian-lai/clean-slate/internal/manifest"
)

func TestAddContextCopiesFiles(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "ctx-target", "add-context test", 0)

	// Create a source doc
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "new-notes.txt")
	if err := os.WriteFile(srcFile, []byte("added later"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeCmd(t, "add-context", "ctx-target", srcFile)
	if err != nil {
		t.Fatalf("add-context: %v", err)
	}

	// File should be in task's context/
	dest := filepath.Join(tasksDir, "ctx-target", "context", "new-notes.txt")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("doc not copied: %v", err)
	}
	if string(data) != "added later" {
		t.Errorf("content = %q, want %q", string(data), "added later")
	}
}

func TestAddContextNonExistentTask(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	srcFile := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeCmd(t, "add-context", "no-such-task", srcFile)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestAddContextUpdatesManifest(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "manifest-update", "test", 0)

	srcFile := filepath.Join(t.TempDir(), "extra.txt")
	if err := os.WriteFile(srcFile, []byte("extra"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := executeCmd(t, "add-context", "manifest-update", srcFile)
	if err != nil {
		t.Fatalf("add-context: %v", err)
	}

	task, err := manifest.Read(filepath.Join(tasksDir, "manifest-update"))
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, d := range task.ContextDocs {
		if d == filepath.Join("context", "extra.txt") {
			found = true
		}
	}
	if !found {
		t.Errorf("manifest ContextDocs missing new entry: %v", task.ContextDocs)
	}
}

func TestAddContextDuplicateFilename(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "dup-filename", "test", 0)

	// Two source files with the same basename, different content
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	f1 := filepath.Join(dir1, "notes.txt")
	f2 := filepath.Join(dir2, "notes.txt")
	if err := os.WriteFile(f1, []byte("first content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("second content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Add first
	if _, _, err := executeCmd(t, "add-context", "dup-filename", f1); err != nil {
		t.Fatalf("first add-context: %v", err)
	}
	// Add second (same basename, different source)
	if _, _, err := executeCmd(t, "add-context", "dup-filename", f2); err != nil {
		t.Fatalf("second add-context: %v", err)
	}

	// Both files should exist in context/ with distinct names
	ctxDir := filepath.Join(tasksDir, "dup-filename", "context")
	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("context/ has %d entries, want 2 (notes.txt + notes_1.txt)", len(entries))
		for _, e := range entries {
			t.Logf("  %s", e.Name())
		}
	}

	// Content should be preserved for both
	haveFirst, haveSecond := false, false
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(ctxDir, e.Name()))
		if err != nil {
			continue
		}
		if string(data) == "first content" {
			haveFirst = true
		}
		if string(data) == "second content" {
			haveSecond = true
		}
	}
	if !haveFirst || !haveSecond {
		t.Errorf("content lost: haveFirst=%v haveSecond=%v", haveFirst, haveSecond)
	}
}

func TestAddContextAlreadyExists(t *testing.T) {
	tasksDir := t.TempDir()
	t.Setenv("CS_TASKS_DIR", tasksDir)

	writeManifest(t, tasksDir, "already-added", "test", 0)

	srcFile := filepath.Join(t.TempDir(), "same.txt")
	if err := os.WriteFile(srcFile, []byte("same"), 0644); err != nil {
		t.Fatal(err)
	}

	// First add succeeds
	if _, _, err := executeCmd(t, "add-context", "already-added", srcFile); err != nil {
		t.Fatalf("first add-context: %v", err)
	}

	// Second add with the SAME source path should skip with a warning
	_, stderr, err := executeCmd(t, "add-context", "already-added", srcFile)
	if err != nil {
		t.Fatalf("second add-context: %v", err)
	}
	if !containsString(stderr, "already") && !containsString(stderr, "skip") {
		t.Errorf("stderr should warn about already-added source: %s", stderr)
	}

	// Manifest should still have exactly one entry for this source
	task, err := manifest.Read(filepath.Join(tasksDir, "already-added"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range task.ContextDocs {
		if d == filepath.Join("context", "same.txt") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ContextDocs has %d entries for same.txt, want 1", count)
	}
}
