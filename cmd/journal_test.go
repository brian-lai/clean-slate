package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brian-lai/clean-slate/cmd"
	"github.com/brian-lai/clean-slate/internal/journal"
)

// TestCreateWritesAndClearsJournal asserts the Phase 1 wire-in contract:
// during cs create, a journal entry exists for the current PID; after
// success, the journal is cleared. The in-memory hook captures the mid-op
// filesystem state deterministically.
func TestCreateWritesAndClearsJournal(t *testing.T) {
	tasksDir, _ := setupTestEnv(t, []string{"repo-journal"})

	taskDir := filepath.Join(tasksDir, "journal-wired")

	// Hook fires right after the first journal.Write inside runCreate.
	// It must observe the journal present on disk for our PID.
	var sawJournal bool
	cmd.SetCreateAfterJournalHook(func() {
		e, ok, err := journal.Read(taskDir, os.Getpid())
		if err != nil {
			t.Errorf("mid-op journal.Read: %v", err)
			return
		}
		if !ok {
			t.Errorf("mid-op journal.Read: expected journal present, got false")
			return
		}
		if e.Op != "create" || e.TaskDir != taskDir || e.PID != os.Getpid() {
			t.Errorf("mid-op journal entry mismatch: %+v", e)
		}
		sawJournal = true
	})
	defer cmd.SetCreateAfterJournalHook(nil)

	_, _, err := executeCmd(t, "create", "journal-wired",
		"--description", "journal wire-in test",
		"--repo", "repo-journal",
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !sawJournal {
		t.Error("hook did not observe journal mid-op — wire-in missing?")
	}

	// Post-success: journal must be cleared.
	_, ok, err := journal.Read(taskDir, os.Getpid())
	if err != nil {
		t.Errorf("post-success journal.Read: %v", err)
	}
	if ok {
		t.Error("journal should be cleared after successful create")
	}
}
