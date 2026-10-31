package cli

import (
	"context"
	"io"
	"os"

	"github.com/zorvok-dev/zorvok-ios/internal/publisher"
)

// runTrunkPush implements `zorvok trunk push [PODSPEC] [--allow-warnings]
// [--skip-lint]`: the Milestone 4 publishing workflow (ARCHITECTURE.md
// §2.4, brief §6) — lint locally, convert to JSON via `pod ipc spec`,
// publish to the configured Zorvok registry.
func runTrunkPush(out, errOut io.Writer, args []string) int {
	var opts publisher.Options
	var podspecPath string
	for _, a := range args {
		switch a {
		case "--allow-warnings":
			opts.AllowWarnings = true
		case "--skip-lint":
			opts.SkipLint = true
		default:
			if podspecPath != "" {
				return exitErr(errOut, "unexpected extra argument %q", a)
			}
			podspecPath = a
		}
	}
	opts.PodspecPath = podspecPath

	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}

	client, url, token, ok := loggedInClient(errOut)
	if !ok {
		return 1
	}

	published, err := publisher.Push(context.Background(), out, errOut, cwd, client, token, opts)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	io.WriteString(out, "Published "+published.Package+" "+published.Version+" (checksum "+published.Checksum+")\n")
	io.WriteString(out, url+"/v1/packages/"+published.Package+"\n")
	return 0
}
