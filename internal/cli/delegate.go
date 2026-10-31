package cli

import (
	"io"
	"os"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
)

// delegateTo returns a command handler that forwards straight to
// `pod <path...> <args...>` in the current directory, streaming output
// live. Per ADR 0002, Zorvok never reimplements a CocoaPods command that
// has no federation- or registry-specific behavior of its own — this is
// the single seam every such pass-through command goes through, so each
// one is a one-line entry in commands() rather than its own file.
func delegateTo(path ...string) func(out, errOut io.Writer, args []string) int {
	return func(out, errOut io.Writer, args []string) int {
		cwd, err := os.Getwd()
		if err != nil {
			return exitErr(errOut, "determine working directory: %v", err)
		}
		full := make([]string, 0, len(path)+len(args))
		full = append(full, path...)
		full = append(full, args...)
		if err := adapter.Run(out, errOut, cwd, full...); err != nil {
			return reportDelegationError(errOut, err)
		}
		return 0
	}
}
