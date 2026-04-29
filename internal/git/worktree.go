package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blai/clean-slate/internal/manifest"
)

// DefaultBranch detects the default branch of a git repo.
// Fallback chain: git symbolic-ref refs/remotes/origin/HEAD → main → master → ErrNoDefaultBranch
func DefaultBranch(repoPath string) (string, error) {
	// Try symbolic-ref first
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err == nil {
		// returns "refs/remotes/origin/main" → "main"
		ref := strings.TrimSpace(string(out))
		parts := strings.Split(ref, "/")
		return parts[len(parts)-1], nil
	}

	// Try checking if main branch exists
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	// Try the current HEAD branch name (for local repos with no remote)
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	out, err = cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" && branch != "HEAD" {
			return branch, nil
		}
	}

	return "", manifest.ErrNoDefaultBranch
}

// AddWorktree creates a git worktree at destPath on a new branch branchName
// based off the repo's default branch. Returns the base branch name.
func AddWorktree(repoPath, destPath, branchName string) (string, error) {
	baseBranch, err := DefaultBranch(repoPath)
	if err != nil {
		return "", fmt.Errorf("detect default branch: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", destPath, "-b", branchName, baseBranch)
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "already checked out") {
			return "", fmt.Errorf("%w: %s", manifest.ErrWorktreeBranchExists, branchName)
		}
		return "", fmt.Errorf("git worktree add: %w\n%s", err, msg)
	}

	return baseBranch, nil
}

// RemoveWorktree removes a git worktree at the given path.
func RemoveWorktree(worktreePath string) error {
	// Find the repo root from the worktree
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}
	// --git-common-dir returns the path to the main .git dir
	gitCommonDir := strings.TrimSpace(string(out))
	// The repo root is the parent of .git
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
		// Check for .git directory or file
		gitPath := filepath.Join(reposDir, e.Name(), ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, e.Name())
		}
	}
	return repos, nil
}
