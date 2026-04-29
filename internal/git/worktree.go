package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrWorktreeBranchExists = errors.New("worktree branch already exists")
	ErrWorktreeDestExists   = errors.New("worktree destination path already exists")
	ErrNoDefaultBranch      = errors.New("could not detect default branch")
)

// DefaultBranch detects the default branch of a git repo.
// Fallback chain: git symbolic-ref refs/remotes/origin/HEAD → main → master → ErrNoDefaultBranch
func DefaultBranch(repoPath string) (string, error) {
	// Try symbolic-ref first (works when origin/HEAD is set)
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		parts := strings.Split(ref, "/")
		return parts[len(parts)-1], nil
	}

	// Fallback: check for main or master
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	return "", ErrNoDefaultBranch
}

// branchExists reports whether a local branch with the given name exists in the repo.
func branchExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// AddWorktree creates a git worktree at destPath on a new branch branchName
// based off the repo's default branch. Returns the base branch name.
func AddWorktree(repoPath, destPath, branchName string) (string, error) {
	// Pre-check: does the branch already exist?
	if branchExists(repoPath, branchName) {
		return "", fmt.Errorf("%w: %s", ErrWorktreeBranchExists, branchName)
	}

	// Pre-check: does the destination path already exist?
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("%w: %s", ErrWorktreeDestExists, destPath)
	}

	baseBranch, err := DefaultBranch(repoPath)
	if err != nil {
		return "", fmt.Errorf("detect default branch: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", destPath, "-b", branchName, baseBranch)
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, stderr.String())
	}

	return baseBranch, nil
}

// RemoveWorktree removes a git worktree at the given path.
func RemoveWorktree(worktreePath string) error {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}
	gitCommonDir := strings.TrimSpace(string(out))
	repoRoot := filepath.Dir(gitCommonDir)

	removeCmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	removeCmd.Dir = repoRoot
	var stderr bytes.Buffer
	removeCmd.Stderr = &stderr
	if err := removeCmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, stderr.String())
	}
	return nil
}

// ListRepos returns the names of directories under reposDir that are git repositories.
func ListRepos(reposDir string) ([]string, error) {
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return nil, fmt.Errorf("read repos dir: %w", err)
	}

	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitPath := filepath.Join(reposDir, e.Name(), ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, e.Name())
		}
	}
	return repos, nil
}
