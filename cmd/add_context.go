package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blai/clean-slate/internal/config"
	"github.com/blai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

var addContextCmd = &cobra.Command{
	Use:   "add-context <name> <path>...",
	Short: "Add supporting documents to an existing task's context/",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runAddContext,
}

func init() {
	rootCmd.AddCommand(addContextCmd)
}

func runAddContext(cmd *cobra.Command, args []string) error {
	useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
	taskName := args[0]
	docPaths := args[1:]

	cfg := config.Load()
	taskDir := filepath.Join(cfg.TasksDir, taskName)

	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		werr := fmt.Errorf("task %q not found at %s", taskName, taskDir)
		outputError(cmd, useJSON, werr)
		return werr
	}

	task, err := manifest.Read(taskDir)
	if err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	contextDir := filepath.Join(taskDir, "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	// Load the set of source paths already recorded for this task.
	// Duplicate detection is by SOURCE path, not destination basename:
	// the same source file added twice is a no-op, but two different sources
	// that happen to share a basename both get saved (with a suffix on the second).
	// Source tracking is persisted via a sidecar JSON so "same source" has meaning
	// across invocations.
	sourceSet, err := readSourceSet(taskDir)
	if err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	// Collect on-disk filenames in context/ for collision-rename
	usedNames := map[string]bool{}
	entries, _ := os.ReadDir(contextDir)
	for _, e := range entries {
		usedNames[e.Name()] = true
	}

	var warnings []string
	var added []string

	for _, src := range docPaths {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", src, err))
			continue
		}

		// Skip if this same source is already tracked
		if sourceSet[absSrc] {
			warnings = append(warnings, fmt.Sprintf("skip %s: already added to this task", absSrc))
			continue
		}

		base := filepath.Base(src)
		finalBase := base
		if usedNames[finalBase] {
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			for n := 1; ; n++ {
				candidate := fmt.Sprintf("%s_%d%s", name, n, ext)
				if !usedNames[candidate] {
					finalBase = candidate
					break
				}
			}
			warnings = append(warnings, fmt.Sprintf("destination filename %q already used, renaming to %q", base, finalBase))
		}

		destPath := filepath.Join(contextDir, finalBase)
		if err := copyFileForAddContext(src, destPath); err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", src, err))
			continue
		}
		usedNames[finalBase] = true
		sourceSet[absSrc] = true

		relFinal := filepath.Join("context", finalBase)
		task.ContextDocs = append(task.ContextDocs, relFinal)
		added = append(added, relFinal)
	}

	// Persist the source set so subsequent add-context calls can dedupe by source.
	if err := writeSourceSet(taskDir, sourceSet); err != nil {
		warnings = append(warnings, fmt.Sprintf("persist source set: %v", err))
	}

	// Re-validate and write
	if err := manifest.Validate(task); err != nil {
		outputError(cmd, useJSON, err)
		return err
	}
	if err := manifest.Write(task, taskDir); err != nil {
		outputError(cmd, useJSON, err)
		return err
	}

	if useJSON {
		result := map[string]any{
			"task":     taskName,
			"added":    added,
			"warnings": warnings,
		}
		if added == nil {
			result["added"] = []string{}
		}
		if warnings == nil {
			result["warnings"] = []string{}
		}
		return outputJSON(cmd, result)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added %d context doc(s) to %q\n", len(added), taskName)
	for _, a := range added {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", a)
	}
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", w)
	}
	return nil
}

// sourceSetPath returns the path to the hidden sidecar file that tracks the
// set of absolute source paths previously added to a task's context/.
// This is how add-context dedupes repeat invocations with the same source.
func sourceSetPath(taskDir string) string {
	return filepath.Join(taskDir, ".cs-sources")
}

// readSourceSet reads the persisted set of absolute source paths (one per line).
// Returns an empty set if the file doesn't exist.
func readSourceSet(taskDir string) (map[string]bool, error) {
	data, err := os.ReadFile(sourceSetPath(taskDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read source set: %w", err)
	}
	set := map[string]bool{}
	for _, line := range splitLines(string(data)) {
		if line != "" {
			set[line] = true
		}
	}
	return set, nil
}

// writeSourceSet overwrites the source set sidecar with the given paths.
func writeSourceSet(taskDir string, set map[string]bool) error {
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	var buf []byte
	for _, p := range paths {
		buf = append(buf, []byte(p+"\n")...)
	}
	return os.WriteFile(sourceSetPath(taskDir), buf, 0644)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// copyFileForAddContext copies a file from src to dest, preserving content.
func copyFileForAddContext(src, dest string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
