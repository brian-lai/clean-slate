package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/brian-lai/clean-slate/internal/manifest"
)

// ErrTaskExists is returned when attempting to create a task whose directory already exists.
var ErrTaskExists = errors.New("task already exists")

// Create creates the task directory structure and copies context docs.
// Returns the task directory path, the relative paths of successfully copied
// context docs, any warnings (skipped files), and an error.
func Create(tasksDir, taskName string, contextPaths []string) (taskDir string, copiedDocs []string, warnings []string, err error) {
	if err := manifest.ValidateName(taskName); err != nil {
		return "", nil, nil, err
	}

	taskDir = filepath.Join(tasksDir, taskName)

	if _, err := os.Stat(taskDir); err == nil {
		return "", nil, nil, fmt.Errorf("%w: %q at %s", ErrTaskExists, taskName, taskDir)
	}

	contextDir := filepath.Join(taskDir, "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return "", nil, nil, fmt.Errorf("create task directory: %w", err)
	}

	// Track destination filenames to detect collisions
	usedNames := map[string]int{}

	for _, src := range contextPaths {
		if src == "" {
			continue
		}
		base := filepath.Base(src)

		// Detect duplicate destination filenames from different source paths
		if count, exists := usedNames[base]; exists {
			// Suffix the filename: notes.txt → notes_1.txt, notes_2.txt, …
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			newBase := fmt.Sprintf("%s_%d%s", name, count, ext)
			warnings = append(warnings, fmt.Sprintf("destination filename %q already used, renaming to %q", base, newBase))
			base = newBase
			usedNames[filepath.Base(src)] = count + 1
		} else {
			usedNames[base] = 1
		}

		dest := filepath.Join(contextDir, base)
		if copyErr := copyFile(src, dest); copyErr != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", src, copyErr))
			continue
		}
		copiedDocs = append(copiedDocs, filepath.Join("context", base))
	}

	return taskDir, copiedDocs, warnings, nil
}

func copyFile(src, dest string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", dest, err)
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return nil
}
