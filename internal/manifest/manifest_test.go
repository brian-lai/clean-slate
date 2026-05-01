package manifest_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brian-lai/clean-slate/internal/manifest"
)

func TestTaskRoundTrip(t *testing.T) {
	task := manifest.Task{
		Name:        "my-task",
		CreatedAt:   time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		JiraTicket:  "PROJ-549",
		Description: "Test task",
		Repos: []manifest.RepoRef{
			{
				Name:           "alpha",
				Source:         "/projects/repos/alpha",
				WorktreePath:   "alpha",
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
	if !errors.Is(err, manifest.ErrDescriptionRequired) {
		t.Errorf("Validate missing description = %v, want ErrDescriptionRequired", err)
	}
}

func TestValidateInvalidTaskNameChars(t *testing.T) {
	invalid := []string{"my task", "feat:thing", "a..b", "a~b", "a^b", "a:b", "a b", "-leading-dash", ".leading-dot"}
	for _, name := range invalid {
		task := manifest.Task{Name: name, Description: "desc"}
		err := manifest.Validate(task)
		if !errors.Is(err, manifest.ErrInvalidTaskName) {
			t.Errorf("Validate name %q = %v, want ErrInvalidTaskName", name, err)
		}
	}

	valid := []string{"my-task", "task_1", "PROJ.549", "a", "my-task-2"}
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
	if !errors.Is(err, manifest.ErrRepoNameRequired) {
		t.Errorf("Validate empty repo name = %v, want ErrRepoNameRequired", err)
	}
}

func TestValidateEmptyRepoSource(t *testing.T) {
	task := manifest.Task{
		Name:        "valid",
		Description: "desc",
		Repos: []manifest.RepoRef{
			{Name: "alpha", Source: ""},
		},
	}
	err := manifest.Validate(task)
	if !errors.Is(err, manifest.ErrRepoSourceRequired) {
		t.Errorf("Validate empty repo source = %v, want ErrRepoSourceRequired", err)
	}
}

func TestReadNonExistent(t *testing.T) {
	_, err := manifest.Read("/nonexistent/path/that/does/not/exist")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Read nonexistent = %v, want os.ErrNotExist", err)
	}
}

// TestWriteConcurrentReadersNeverSeePartial asserts the durability claim of
// manifest.Write: a reader watching the file while a writer repeatedly
// overwrites it should never observe a truncated or half-parsed JSON. After
// Phase 1's atomicio swap, readers only see fully-new or fully-old content.
func TestWriteConcurrentReadersNeverSeePartial(t *testing.T) {
	dir := t.TempDir()

	// Seed with valid content.
	base := manifest.Task{
		Name:        "race",
		CreatedAt:   time.Now().UTC(),
		Description: "race test",
		Repos:       []manifest.RepoRef{},
		ContextDocs: []string{},
	}
	if err := manifest.Write(base, dir); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	failures := make(chan error, 100)

	// Reader goroutine: repeatedly parse task.json; any parse error is a failure.
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if _, err := manifest.Read(dir); err != nil {
					failures <- err
				}
			}
		}
	}()

	// Writer: 100 overwrites with distinct descriptions.
	for i := 0; i < 100; i++ {
		task := base
		task.Description = "write " + itoaManifest(i)
		if err := manifest.Write(task, dir); err != nil {
			close(done)
			t.Fatalf("write i=%d: %v", i, err)
		}
	}
	close(done)

	select {
	case err := <-failures:
		t.Errorf("reader observed partial JSON during concurrent writes: %v", err)
	default:
		// no observed failures
	}

	// No tempfiles left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || contains(e.Name(), ".tmp-") {
			t.Errorf("leftover tempfile: %s", e.Name())
		}
	}
}

func itoaManifest(n int) string {
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

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
