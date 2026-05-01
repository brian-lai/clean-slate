//go:build darwin || linux

// Package cslock provides a non-blocking, OS-level advisory lock around a
// single cs operation on a single task. Lock is per-task: independent tasks
// proceed in parallel; same-task invocations fail-fast with ErrLocked so
// agents can decide to retry.
//
// Supported platforms: darwin, linux. On other platforms, see cslock_windows.go
// which returns ErrUnsupportedPlatform from every entry point.
//
// Callers compose the lock path as `<tasksDir>/.cs-locks/<taskname>.lock`.
// Acquire MkdirAlls the parent dir; Release cleans up the lock file.
package cslock

import (
	"errors"
	"time"
)

// LockInfo is the payload persisted to the lock file so contenders can report
// who is holding the lock. PID == -1 indicates "unknown holder" (reader raced
// writer's truncate-then-write window).
type LockInfo struct {
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
}

// ErrLocked wraps the info of the current holder when Acquire loses the race.
type ErrLocked struct {
	Info LockInfo
}

func (e *ErrLocked) Error() string {
	return "cslock: lock is held by another process"
}

// ErrUnsupportedPlatform is returned by the stubs on platforms where flock
// is not supported (currently: anything non-darwin, non-linux).
var ErrUnsupportedPlatform = errors.New("cslock: platform not supported (darwin and linux only)")

// Lock is an acquired advisory lock. Callers must call Release exactly once
// (typically via defer).
type Lock struct{}

// Acquire attempts a non-blocking exclusive flock on lockPath. Implementation
// lands in Phase 2.
func Acquire(lockPath string) (*Lock, error) {
	return nil, errors.New("cslock.Acquire: not implemented")
}

// Release releases the underlying flock, closes the fd, and removes the lock
// file. Idempotent.
func (l *Lock) Release() error {
	return errors.New("cslock.Release: not implemented")
}
