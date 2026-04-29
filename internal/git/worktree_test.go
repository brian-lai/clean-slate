package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blai/clean-slate/internal/git"
)

// initRepo creates a temp git repo with an initial commit on the given branch.
func initRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", branch)
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	// Create an initial commit so the branch exists
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func TestDefaultBranch(t *testing.T) {
	repo := initRepo(t, "main")
	branch, err := git.DefaultBranch(repo)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", branch, "main")
	}
}

func TestDefaultBranchFallback(t *testing.T) {
	repo := initRepo(t, "master")
	branch, err := git.DefaultBranch(repo)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "master" {
		t.Errorf("DefaultBranch = %q, want %q", branch, "master")
	}
}

// TestDefaultBranchFromOriginHEAD verifies the primary symbolic-ref path
// by cloning a "remote" repo, which causes origin/HEAD to be set.
func TestDefaultBranchFromOriginHEAD(t *testing.T) {
	// Create a source repo with "trunk" as the default branch
	source := initRepo(t, "trunk")

	// Clone it to another dir; git clone sets origin/HEAD → trunk
	cloneDir := t.TempDir()
	clonePath := filepath.Join(cloneDir, "clone")
	cmd := exec.Command("git", "clone", source, clonePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	branch, err := git.DefaultBranch(clonePath)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "trunk" {
		t.Errorf("DefaultBranch = %q, want %q (from origin/HEAD)", branch, "trunk")
	}
}

// TestDefaultBranchNone verifies ErrNoDefaultBranch is returned when
// origin/HEAD is unset AND neither main nor master exist.
func TestDefaultBranchNone(t *testing.T) {
	// Init repo on a non-standard branch
	repo := initRepo(t, "feature/weird")

	_, err := git.DefaultBranch(repo)
	if !errors.Is(err, git.ErrNoDefaultBranch) {
		t.Errorf("DefaultBranch on repo with no main/master = %v, want ErrNoDefaultBranch", err)
	}
}

func TestWorktreeAdd(t *testing.T) {
	repo := initRepo(t, "main")
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "my-worktree")

	baseBranch, err := git.AddWorktree(repo, dest, "ws/my-task")
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if baseBranch != "main" {
		t.Errorf("baseBranch = %q, want %q", baseBranch, "main")
	}

	// Worktree dir should exist
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("worktree dir not created: %v", err)
	}

	// Branch in worktree should be ws/my-task
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dest
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch: %v", err)
	}
	got := string(out[:len(out)-1]) // trim newline
	if got != "ws/my-task" {
		t.Errorf("branch = %q, want %q", got, "ws/my-task")
	}
}

func TestWorktreeAddBranchExists(t *testing.T) {
	repo := initRepo(t, "main")
	destDir := t.TempDir()

	// Pre-create the branch
	cmd := exec.Command("git", "branch", "ws/my-task")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pre-create branch: %v\n%s", err, out)
	}

	_, err := git.AddWorktree(repo, filepath.Join(destDir, "wt"), "ws/my-task")
	if !errors.Is(err, git.ErrWorktreeBranchExists) {
		t.Errorf("AddWorktree with existing branch = %v, want ErrWorktreeBranchExists", err)
	}
}

func TestWorktreeRemove(t *testing.T) {
	repo := initRepo(t, "main")
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "my-worktree")

	if _, err := git.AddWorktree(repo, dest, "ws/remove-test"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	if err := git.RemoveWorktree(dest); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after remove")
	}
}

func TestListRepos(t *testing.T) {
	reposDir := t.TempDir()

	for _, name := range []string{"repo-a", "repo-b"} {
		dir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "init", "-b", "main", dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v\n%s", name, err, out)
		}
	}

	// Create 1 non-git directory
	nonGit := filepath.Join(reposDir, "not-a-repo")
	if err := os.MkdirAll(nonGit, 0755); err != nil {
		t.Fatal(err)
	}

	repos, err := git.ListRepos(reposDir)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("ListRepos = %v (len %d), want 2 repos", repos, len(repos))
	}
	for _, r := range repos {
		if r == "not-a-repo" {
			t.Errorf("ListRepos included non-git dir %q", r)
		}
	}
}
