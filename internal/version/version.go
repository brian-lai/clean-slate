// Package version exposes the build-time injected version of the cs binary.
// The Version variable defaults to "dev" and is overridden via ldflags at build time:
//
//	go build -ldflags "-X github.com/brian-lai/clean-slate/internal/version.Version=v0.1.0"
package version

// Version is the build-time injected version string.
// Defaults to "dev" when built without ldflags injection.
var Version = "dev"
