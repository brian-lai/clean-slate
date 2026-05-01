package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestE2EAgentRaceSmoke spawns two `cs create` subprocesses racing on the
// same task name and asserts the Phase 2 concurrency contract:
//   - exactly one exits 0 with valid JSON success,
//   - the other exits 1 with stderr JSON containing "is locked by PID",
//   - no partial residue on disk (exactly one task dir).
//
// This test exists as a skeleton from Phase 2 step 1 and is skipped until
// step 9 wires the lock into runCreate. The skip is removed in that commit.
func TestE2EAgentRaceSmoke(t *testing.T) {
	// Build the binary to a tempdir.
	repoRoot := findRepoRoot(t)
	binPath := filepath.Join(t.TempDir(), "cs")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cs: %v\n%s", err, out)
	}

	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	repoName := "repo-race"
	setupSourceRepo(t, filepath.Join(reposDir, repoName))

	env := append(os.Environ(),
		"CS_TASKS_DIR="+tasksDir,
		"CS_REPOS_DIR="+reposDir,
	)

	type result struct {
		stdout string
		stderr string
		err    error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c := exec.Command(binPath, "create", "race-target",
				"--description", "race test",
				"--repo", repoName,
				"--json",
			)
			c.Env = env
			var out, errb strings.Builder
			c.Stdout = &out
			c.Stderr = &errb
			err := c.Run()
			results <- result{out.String(), errb.String(), err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var succeeded, failed int
	for r := range results {
		if r.err == nil {
			succeeded++
			// stdout should parse as success JSON
			var payload map[string]any
			if jerr := json.Unmarshal([]byte(r.stdout), &payload); jerr != nil {
				t.Errorf("success case stdout not valid JSON: %v\n%s", jerr, r.stdout)
			}
		} else {
			failed++
			// stderr should contain lock-held JSON
			if !strings.Contains(r.stderr, "is locked by PID") {
				t.Errorf("failed case stderr missing lock-held signal: %q", r.stderr)
			}
			var payload map[string]any
			if jerr := json.Unmarshal([]byte(r.stderr), &payload); jerr != nil {
				t.Errorf("failed case stderr not valid JSON: %v\n%s", jerr, r.stderr)
			}
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("race result: %d succeeded, %d failed; want 1+1", succeeded, failed)
	}

	// Exactly one task dir on disk; no residue from the loser.
	entries, _ := os.ReadDir(tasksDir)
	var taskCount int
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			taskCount++
		}
	}
	if taskCount != 1 {
		t.Errorf("expected exactly 1 task dir, got %d", taskCount)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	t.Fatal("could not find go.mod")
	return ""
}

func setupSourceRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

// compile-time: suppress unused-import warnings when skip is active
var _ = fmt.Sprintf
