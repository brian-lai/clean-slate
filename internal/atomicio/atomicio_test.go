package atomicio_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brian-lai/clean-slate/internal/atomicio"
)

// TestWriteFileAtomic verifies that after WriteFile the path holds exactly the
// new content with no tempfile siblings. Repeats 50× to catch naming / cleanup
// regressions under repetition.
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")

	for i := 0; i < 50; i++ {
		content := []byte("iteration " + itoa(i))
		if err := atomicio.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("WriteFile i=%d: %v", i, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile i=%d: %v", i, err)
		}
		if string(got) != string(content) {
			t.Errorf("i=%d: got %q, want %q", i, got, content)
		}
		// No tempfile siblings should linger on the happy path.
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.Contains(e.Name(), ".tmp-") {
				t.Errorf("i=%d: leftover tempfile %q", i, e.Name())
			}
		}
	}
}

// TestWriteFileCleansTempOnError verifies that a failing write leaves no
// tempfile siblings behind. We force a failure by pointing at a path under
// a directory that doesn't exist — OpenFile on the temp path itself will fail.
func TestWriteFileCleansTempOnError(t *testing.T) {
	dir := t.TempDir()
	// Target a nested path whose parent doesn't exist.
	path := filepath.Join(dir, "nonexistent-subdir", "target.json")

	err := atomicio.WriteFile(path, []byte("x"), 0644)
	if err == nil {
		t.Fatal("WriteFile to missing parent: expected error, got nil")
	}

	// dir should still be empty — no half-written tempfile, no subdir.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir should be empty, found: %v", names)
	}
}

// TestWriteFilePreservesPerm verifies the mode bits are respected.
func TestWriteFilePreservesPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")

	if err := atomicio.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %o, want 0600", info.Mode().Perm())
	}
}

// TestWriteFileOverwritesExisting verifies that WriteFile replaces an existing
// file (rename-over-existing works on POSIX).
func TestWriteFileOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")

	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := atomicio.WriteFile(path, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("got %q, want %q", got, "new")
	}
}

// itoa keeps the test free of strconv imports for readability; kept small.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
