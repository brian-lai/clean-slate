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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
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
	if e.Info.PID == -1 {
		return "cslock: lock is held by an unknown process"
	}
	return fmt.Sprintf("cslock: lock is held by PID %d", e.Info.PID)
}

// ErrUnsupportedPlatform is returned by the stubs on platforms where flock
// is not supported (currently: anything non-darwin, non-linux).
var ErrUnsupportedPlatform = errors.New("cslock: platform not supported (darwin and linux only)")

// Lock is an acquired advisory lock. Callers must call Release exactly once
// (typically via defer).
type Lock struct {
	fd       *os.File
	path     string
	released bool
	mu       sync.Mutex
}

// Acquire attempts a non-blocking exclusive flock on lockPath. MkdirAlls the
// parent dir. On success: writes a LockInfo JSON payload and returns *Lock.
// On contention: returns *ErrLocked with the current holder's info; if the
// payload is empty or unparseable (race with writer's truncate), returns
// *ErrLocked{Info: LockInfo{PID: -1}} after a single 10ms retry.
func Acquire(lockPath string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("cslock: mkdir parent: %w", err)
	}

	// O_CREATE|O_RDWR (no O_TRUNC) preserves the existing payload so contenders
	// can read the current holder's info.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("cslock: open: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Contention. Read payload, close fd, return ErrLocked.
		info := readLockInfoWithRetry(lockPath)
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, &ErrLocked{Info: info}
		}
		return nil, fmt.Errorf("cslock: flock: %w", err)
	}

	// Success. Write our payload.
	if err := f.Truncate(0); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		return nil, fmt.Errorf("cslock: truncate: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		return nil, fmt.Errorf("cslock: seek: %w", err)
	}
	payload, _ := json.Marshal(LockInfo{PID: os.Getpid(), Started: time.Now().UTC()})
	if _, err := f.Write(payload); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		return nil, fmt.Errorf("cslock: write payload: %w", err)
	}
	_ = f.Sync()

	return &Lock{fd: f, path: lockPath}, nil
}

// readLockInfoWithRetry reads and parses LockInfo from lockPath, retrying
// once after 10ms if the first read yields an empty or unparseable payload.
// Returns LockInfo{PID: -1} if both attempts fail.
func readLockInfoWithRetry(lockPath string) LockInfo {
	for attempt := 0; attempt < 2; attempt++ {
		data, err := os.ReadFile(lockPath)
		if err == nil && len(data) > 0 {
			var info LockInfo
			if json.Unmarshal(data, &info) == nil {
				return info
			}
		}
		if attempt == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return LockInfo{PID: -1}
}

// Release releases the underlying flock, closes the fd, and removes the lock
// file. Idempotent.
func (l *Lock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return nil
	}
	l.released = true

	_ = syscall.Flock(int(l.fd.Fd()), syscall.LOCK_UN)
	_ = l.fd.Close()
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cslock: remove lock file: %w", err)
	}
	return nil
}
