package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/blai/clean-slate/internal/manifest"
)

var validTaskName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var invalidTaskNamePatterns = regexp.MustCompile(`\.\.`)

// Create creates the task directory structure and copies context docs.
// Returns the task directory path, any warnings (skipped files), and an error.
func Create(tasksDir, taskName string, contextPaths []string) (taskDir string, warnings []string, err error) {
	if taskName == "" || !validTaskName.MatchString(taskName) || invalidTaskNamePatterns.MatchString(taskName) {
		return "", nil, fmt.Errorf("%w: %q", manifest.ErrInvalidTaskName, taskName)
	}

	taskDir = filepath.Join(tasksDir, taskName)

	if _, err := os.Stat(taskDir); err == nil {
		return "", nil, fmt.Errorf("%w: %q at %s", manifest.ErrTaskExists, taskName, taskDir)
	}

	if err := os.MkdirAll(filepath.Join(taskDir, "context"), 0755); err != nil {
		return "", nil, fmt.Errorf("create task directory: %w", err)
	}

	for _, src := range contextPaths {
		if src == "" {
			continue
		}
		w, copyErr := copyFile(src, filepath.Join(taskDir, "context", filepath.Base(src)))
		if w != "" {
			warnings = append(warnings, w)
		}
		if copyErr != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", src, copyErr))
		}
	}

	return taskDir, warnings, nil
}

func copyFile(src, dest string) (warning string, err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Sprintf("skip %s: %v", src, err), nil
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create dest %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return "", fmt.Errorf("copy %s: %w", src, err)
	}
	return "", nil
}
