package cmd_test

// Concurrency tests run cs as a subprocess because the in-process test harness
// (via cmd.ExecuteArgs) shares cobra's global flag state across goroutines,
// which would race. Subprocess isolation is closer to how agents actually use
// cs anyway.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// buildCS builds the cs binary once per test and returns its path.
func buildCS(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	var repoRoot string
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			repoRoot = dir
			break
		}
	}
	if repoRoot == "" {
		t.Fatal("could not find go.mod")
	}
	bin := filepath.Join(t.TempDir(), "cs")
	c := exec.Command("go", "build", "-o", bin, ".")
	c.Dir = repoRoot
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// initGitRepo sets up a minimal git repo under dir.
func initGitRepo(t *testing.T, dir string) {
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

// TestConcurrentCreateSerializes fires two `cs create` subprocesses on the
// same task name in parallel. Exactly one must succeed; the other must fail
// with the lock-held error. No partial residue on disk.
func TestConcurrentCreateSerializes(t *testing.T) {
	bin := buildCS(t)
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initGitRepo(t, filepath.Join(reposDir, "repo-race"))

	env := append(os.Environ(),
		"CS_TASKS_DIR="+tasksDir,
		"CS_REPOS_DIR="+reposDir,
	)

	type result struct {
		stdout, stderr string
		err            error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c := exec.Command(bin, "create", "race-target",
				"--description", "race",
				"--repo", "repo-race",
				"--json",
			)
			c.Env = env
			var out, errb bytes.Buffer
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
		} else {
			failed++
			if !strings.Contains(r.stderr, "is locked by PID") {
				t.Errorf("failed case: stderr missing 'is locked by PID': %q", r.stderr)
			}
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("got %d successes, %d failures; want 1+1", succeeded, failed)
	}

	entries, _ := os.ReadDir(tasksDir)
	var visibleNames []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			visibleNames = append(visibleNames, e.Name())
		}
	}
	if len(visibleNames) != 1 {
		t.Errorf("expected exactly 1 visible task dir, got %d: %v", len(visibleNames), visibleNames)
	}
}

// TestIndependentTasksUnblocked verifies two different task names create in
// parallel without blocking each other.
func TestIndependentTasksUnblocked(t *testing.T) {
	bin := buildCS(t)
	tasksDir := t.TempDir()
	reposDir := t.TempDir()
	initGitRepo(t, filepath.Join(reposDir, "repo-a"))
	initGitRepo(t, filepath.Join(reposDir, "repo-b"))

	env := append(os.Environ(),
		"CS_TASKS_DIR="+tasksDir,
		"CS_REPOS_DIR="+reposDir,
	)

	run := func(name, repoName string) error {
		c := exec.Command(bin, "create", name,
			"--description", name,
			"--repo", repoName,
			"--json",
		)
		c.Env = env
		return c.Run()
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(1)
	go func() { defer wg.Done(); errs <- run("task-a", "repo-a") }()
	wg.Add(1)
	go func() { defer wg.Done(); errs <- run("task-b", "repo-b") }()
	wg.Wait()
	close(errs)

	for e := range errs {
		if e != nil {
			t.Errorf("independent task failed: %v", e)
		}
	}
}
