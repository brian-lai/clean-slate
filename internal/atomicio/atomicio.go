// Package atomicio implements durable, crash-safe file writes via the
// temp-file + fsync + rename + dir-fsync pattern. A reader of `path` after a
// successful WriteFile sees either the fully-new or fully-old content — never
// a truncated or partial write.
//
// Durability bar: survive SIGKILL / panic mid-write. Not claiming durability
// against power loss on filesystems that ignore fsync (some tmpfs, NFSv3
// without rpc.lockd); that's orthogonal to the crash-safety goal.
package atomicio

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically writes data to path with the given permissions.
// Algorithm:
//  1. Write data to a sibling tempfile `<dir>/.<base>.tmp-<pid>-<rand>`.
//  2. fsync the tempfile, close it.
//  3. os.Rename tempfile to path (atomic on POSIX within a single filesystem).
//  4. fsync the containing directory so the rename is durable.
//
// On any error after tempfile creation, the tempfile is removed and the error
// is returned wrapped with path context. Never leaves a tempfile behind on
// success. May leave a tempfile on a crash between tempfile-write and rename;
// an orphan sweep (Phase 2 of v0.2.0) handles that residue.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	suffix, err := randSuffix()
	if err != nil {
		return fmt.Errorf("atomicio: generate tempfile suffix for %s: %w", path, err)
	}
	tmpPath := filepath.Join(dir, fmt.Sprintf(".%s.tmp-%d-%s", base, os.Getpid(), suffix))

	// O_EXCL rejects the astronomical-but-nonzero case of tempfile-name collision.
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("atomicio: open tempfile for %s: %w", path, err)
	}

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomicio: write tempfile for %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomicio: fsync tempfile for %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomicio: close tempfile for %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("atomicio: rename tempfile for %s: %w", path, err)
	}

	// fsync the containing directory so the rename itself is durable. On a
	// POSIX FS this ensures a subsequent readdir sees `path` and not `tmpPath`.
	// tmpfs and some network mounts may no-op this; we issue it for the FSes
	// that honor it and accept the no-op cost on those that don't.
	if dirF, err := os.Open(dir); err == nil {
		_ = dirF.Sync()
		_ = dirF.Close()
	}
	return nil
}

func randSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
