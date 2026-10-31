package cli

import (
	"io"
	"os"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/podspec"
)

func runLibLint(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}
	if err := podspec.Lint(out, errOut, cwd, args); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}
