// Package atomicio implements durable, crash-safe file writes via the
// temp-file + fsync + rename + dir-fsync pattern. A reader of `path` after a
// successful WriteFile sees either the fully-new or fully-old content — never
// a truncated or partial write.
//
// Stub: signature matches the spec at
// context/data/2026-05-01-durable-concurrent-cs-spec.yaml. Implementation
// lands in Phase 1 of the durable-concurrent-cs plan.
package atomicio

import (
	"errors"
	"os"
)

// WriteFile atomically writes data to path with the given permissions.
// See package doc for the guarantee.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return errors.New("atomicio.WriteFile: not implemented")
}
