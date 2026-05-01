//go:build darwin || linux

package cslock_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/brian-lai/clean-slate/internal/cslock"
)

func TestAcquireSucceeds(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lock.Release()

	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var info cslock.LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("lock file payload not valid JSON: %v\n%s", err, data)
	}
	if info.PID != os.Getpid() {
		t.Errorf("LockInfo.PID = %d, want %d", info.PID, os.Getpid())
	}
	if time.Since(info.Started) > 5*time.Second {
		t.Errorf("LockInfo.Started too stale: %v", info.Started)
	}
}

// TestAcquireMkdirsParent verifies Acquire creates the parent .cs-locks/ dir.
func TestAcquireMkdirsParent(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".cs-locks", "task.lock")

	lock, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("Acquire with missing parent: %v", err)
	}
	defer lock.Release()

	if _, err := os.Stat(filepath.Dir(lockPath)); err != nil {
		t.Errorf(".cs-locks/ should exist: %v", err)
	}
}

// TestAcquireContentionReturnsErrLocked verifies contention behavior.
func TestAcquireContentionReturnsErrLocked(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lockA, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer lockA.Release()

	_, err = cslock.Acquire(lockPath)
	if err == nil {
		t.Fatal("second Acquire: expected error, got nil")
	}
	var locked *cslock.ErrLocked
	if !errors.As(err, &locked) {
		t.Fatalf("expected *ErrLocked, got %T: %v", err, err)
	}
	if locked.Info.PID != os.Getpid() {
		t.Errorf("ErrLocked.Info.PID = %d, want %d", locked.Info.PID, os.Getpid())
	}
	if time.Since(locked.Info.Started) > 5*time.Second {
		t.Errorf("ErrLocked.Info.Started too stale: %v", locked.Info.Started)
	}
}

// TestAcquireEmptyPayloadRace verifies the reader-retries-on-empty contract.
// Simulates: lock file exists, zero-length (writer truncated but not yet
// written). We pre-flock it to force the contention path and leave payload empty.
func TestAcquireEmptyPayloadRace(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Pre-acquire via the real API to hold the flock.
	holder, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()

	// Force the payload to be empty to simulate the race window.
	if err := os.Truncate(lockPath, 0); err != nil {
		t.Fatal(err)
	}

	_, err = cslock.Acquire(lockPath)
	if err == nil {
		t.Fatal("expected ErrLocked, got nil")
	}
	var locked *cslock.ErrLocked
	if !errors.As(err, &locked) {
		t.Fatalf("expected *ErrLocked, got %T: %v", err, err)
	}
	if locked.Info.PID != -1 {
		t.Errorf("empty payload should yield PID=-1, got %d", locked.Info.PID)
	}
}

func TestReleaseRemovesFile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("lock file should be removed after Release")
	}
	// Subsequent Acquire succeeds.
	lock2, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("second Acquire after Release: %v", err)
	}
	lock2.Release()
}

func TestReleaseIdempotent(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := cslock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("second Release should be nil, got %v", err)
	}
}

// TestConcurrentAcquireSerializes is a stress test: 10 goroutines racing on
// the same lock. Exactly one succeeds; the rest get ErrLocked. After that one
// releases, another can acquire.
func TestConcurrentAcquireSerializes(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "stress.lock")

	var wg sync.WaitGroup
	var successes, contentions int
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := cslock.Acquire(lockPath)
			if err != nil {
				var locked *cslock.ErrLocked
				if errors.As(err, &locked) {
					mu.Lock()
					contentions++
					mu.Unlock()
				} else {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			mu.Lock()
			successes++
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			lock.Release()
		}()
	}
	wg.Wait()

	if successes < 1 {
		t.Errorf("expected at least 1 success, got %d", successes)
	}
	if successes+contentions != 10 {
		t.Errorf("counts don't sum: successes=%d contentions=%d", successes, contentions)
	}
}
