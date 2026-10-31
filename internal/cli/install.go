package cli

import (
	"errors"
	"io"
	"os"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/adapter"
	"github.com/zorvok-dev/zorvok/engines/cocoapods/installer"
)

func runInstall(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := installer.Install(out, errOut, cwd, args); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

func runUpdate(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := installer.Update(out, errOut, cwd, args); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// reportDelegationError prints a friendly message for the preflight
// failures (no Podfile, pod not found) we check for ourselves, and a
// plain passthrough for anything that bubbled up from `pod` itself
// (`pod` already prints its own diagnostics to errOut via Run).
func reportDelegationError(errOut io.Writer, err error) int {
	if errors.Is(err, adapter.ErrCommandFailed) {
		// pod itself already wrote its own diagnostics to errOut; a
		// "zorvok: " wrapper here would just be redundant noise.
		return 1
	}
	return exitErr(errOut, "%v", err)
}
