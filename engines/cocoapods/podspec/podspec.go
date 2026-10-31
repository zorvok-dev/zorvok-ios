// Package podspec implements `zorvok lib lint` for Milestone 2 by
// delegating to `pod lib lint`, the real podspec validator (syntax,
// required metadata, platform/deployment-target checks, and a trial
// build). Per ARCHITECTURE.md §2.4, podspec lint stays entirely
// client-side; the registry server never executes package code.
package podspec

import (
	"io"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/adapter"
)

// Lint runs `pod lib lint` in dir, forwarding extraArgs verbatim
// (e.g. --allow-warnings, a specific .podspec path).
func Lint(out, errOut io.Writer, dir string, extraArgs []string) error {
	if _, err := adapter.Find(); err != nil {
		return err
	}
	return adapter.Run(out, errOut, dir, append([]string{"lib", "lint"}, extraArgs...)...)
}

// JSON returns a podspec's CocoaPods-Core representation as JSON, via
// `pod ipc spec` — the same IPC seam ARCHITECTURE.md §2.2 designates as
// the only place a podspec's Ruby DSL gets evaluated. Used by
// `zorvok trunk push` (internal/publisher) to get a declarative,
// publishable form of the podspec without Zorvok ever parsing Ruby
// itself.
func JSON(errOut io.Writer, dir, podspecPath string) (string, error) {
	if _, err := adapter.Find(); err != nil {
		return "", err
	}
	return adapter.Capture(errOut, dir, "ipc", "spec", podspecPath)
}
