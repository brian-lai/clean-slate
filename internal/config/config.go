package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	TasksDir string
	ReposDir string
}

func Load() Config {
	home, _ := os.UserHomeDir()
	return Config{
		TasksDir: resolve(os.Getenv("CS_TASKS_DIR"), filepath.Join(home, "projects", "tasks"), home),
		ReposDir: resolve(os.Getenv("CS_REPOS_DIR"), filepath.Join(home, "projects", "repos"), home),
	}
}

func resolve(envVal, defaultVal, home string) string {
	if envVal == "" {
		return defaultVal
	}
	if strings.HasPrefix(envVal, "~/") {
		return filepath.Join(home, envVal[2:])
	}
	return envVal
}
