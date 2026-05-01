//go:build windows

// Windows stub for internal/cslock. cs is darwin/linux-only for v0.2.0;
// the stub keeps the package cross-compilable so unrelated code can still
// `GOOS=windows go build ./...`. Every entry point returns
// ErrUnsupportedPlatform so runtime is explicit about the gap.
package cslock

import (
	"errors"
	"time"
)

type LockInfo struct {
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
}

type ErrLocked struct {
	Info LockInfo
}

func (e *ErrLocked) Error() string {
	return "cslock: lock is held by another process"
}

var ErrUnsupportedPlatform = errors.New("cslock: platform not supported (darwin and linux only)")

type Lock struct{}

func Acquire(lockPath string) (*Lock, error) {
	return nil, ErrUnsupportedPlatform
}

func (l *Lock) Release() error {
	return ErrUnsupportedPlatform
}
