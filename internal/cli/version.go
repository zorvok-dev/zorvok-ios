package cli

import (
	"fmt"
	"io"
	"runtime"
)

// Version is the Zorvok release version. Overridden at build time via
// `-ldflags "-X github.com/zorvok-dev/zorvok-ios/internal/cli.Version=v0.1.0"`;
// defaults to "dev" for local builds.
var Version = "dev"

func runVersion(out, _ io.Writer, _ []string) int {
	fmt.Fprintf(out, "zorvok %s (%s, %s/%s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return 0
}
