package version_test

import (
	"testing"

	"github.com/blai/clean-slate/internal/version"
)

func TestVersionDefault(t *testing.T) {
	// Without -ldflags injection, Version should be "dev".
	if version.Version != "dev" {
		t.Errorf("Version = %q, want %q (default value when not injected)", version.Version, "dev")
	}
}
