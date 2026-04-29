package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blai/clean-slate/internal/config"
)

func TestDefaultPaths(t *testing.T) {
	// Clear env vars to test defaults
	t.Setenv("CS_TASKS_DIR", "")
	t.Setenv("CS_REPOS_DIR", "")

	cfg := config.Load()
	home, _ := os.UserHomeDir()

	want := filepath.Join(home, "projects", "tasks")
	if cfg.TasksDir != want {
		t.Errorf("TasksDir = %q, want %q", cfg.TasksDir, want)
	}

	want = filepath.Join(home, "projects", "repos")
	if cfg.ReposDir != want {
		t.Errorf("ReposDir = %q, want %q", cfg.ReposDir, want)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("CS_TASKS_DIR", "/tmp/my-tasks")
	t.Setenv("CS_REPOS_DIR", "/tmp/my-repos")

	cfg := config.Load()

	if cfg.TasksDir != "/tmp/my-tasks" {
		t.Errorf("TasksDir = %q, want %q", cfg.TasksDir, "/tmp/my-tasks")
	}
	if cfg.ReposDir != "/tmp/my-repos" {
		t.Errorf("ReposDir = %q, want %q", cfg.ReposDir, "/tmp/my-repos")
	}
}

func TestTildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("CS_TASKS_DIR", "~/foo/tasks")
	t.Setenv("CS_REPOS_DIR", "~/bar/repos")

	cfg := config.Load()

	wantTasks := filepath.Join(home, "foo", "tasks")
	wantRepos := filepath.Join(home, "bar", "repos")

	if cfg.TasksDir != wantTasks {
		t.Errorf("TasksDir = %q, want %q", cfg.TasksDir, wantTasks)
	}
	if cfg.ReposDir != wantRepos {
		t.Errorf("ReposDir = %q, want %q", cfg.ReposDir, wantRepos)
	}
}
