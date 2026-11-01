package cli

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/zorvok-dev/zorvok-ios/internal/auth"
	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

type checkStatus int

const (
	statusOK checkStatus = iota
	statusWarn
	statusFail
)

type checkResult struct {
	status checkStatus
	name   string
	detail string
}

// runDoctor diagnoses the local environment. Ruby/CocoaPods checks are
// reported as warnings rather than failures — `zorvok install` falls
// back to delegating to a real `pod install`, so a missing Ruby/
// CocoaPods toolchain doesn't block every command, only that one.
func runDoctor(out, errOut io.Writer, _ []string) int {
	var results []checkResult

	results = append(results, checkPlatform())
	results = append(results, checkTool("git", "git", "git --version", "needed for git: dependencies", statusWarn))
	results = append(results, checkTool("ruby", "ruby", "ruby -v", "needed to run `pod install`", statusWarn))
	results = append(results, checkCocoaPods())
	results = append(results, checkConfigDir())
	results = append(results, checkAuth())

	failed := false
	for _, r := range results {
		symbol := "✓"
		switch r.status {
		case statusWarn:
			symbol = "!"
		case statusFail:
			symbol = "✗"
			failed = true
		}
		if r.detail != "" {
			fmt.Fprintf(out, "%s %-20s %s\n", symbol, r.name, r.detail)
		} else {
			fmt.Fprintf(out, "%s %s\n", symbol, r.name)
		}
	}

	fmt.Fprintln(out)
	if failed {
		fmt.Fprintln(out, "zorvok doctor found problems that need attention.")
		return 1
	}
	fmt.Fprintln(out, "zorvok doctor found no blocking problems.")
	return 0
}

func checkPlatform() checkResult {
	switch runtime.GOOS {
	case "darwin", "linux":
		return checkResult{statusOK, "platform", fmt.Sprintf("%s/%s (supported)", runtime.GOOS, runtime.GOARCH)}
	default:
		return checkResult{statusFail, "platform", fmt.Sprintf("%s/%s (unsupported — macOS and Linux only)", runtime.GOOS, runtime.GOARCH)}
	}
}

// checkTool verifies binaryName is on PATH and reports its version
// output under displayName (which may differ — `pod` is reported as
// "cocoapods"). versionPrefix is prepended to an OK result's version
// string, for tools like `pod --version` whose output is a bare number
// with no "git version"/"ruby" style prefix of its own.
func checkTool(binaryName, displayName, versionCmd, missingHint string, missingSeverity checkStatus) checkResult {
	path, err := exec.LookPath(binaryName)
	if err != nil {
		return checkResult{missingSeverity, displayName, "not found on PATH — " + missingHint}
	}

	parts := strings.Fields(versionCmd)
	out, err := exec.Command(parts[0], parts[1:]...).Output()
	version := strings.TrimSpace(string(out))
	switch {
	case err != nil:
		return checkResult{statusWarn, displayName, fmt.Sprintf("%s (found, but %q failed: %v)", path, versionCmd, err)}
	case version == "":
		return checkResult{statusWarn, displayName, path + " (found, but couldn't read version)"}
	default:
		return checkResult{statusOK, displayName, firstLine(version) + "  (" + path + ")"}
	}
}

func checkCocoaPods() checkResult {
	r := checkTool("pod", "cocoapods", "pod --version", "needed for `zorvok install`; install with `gem install cocoapods`", statusWarn)
	if r.status == statusOK {
		r.detail = "v" + r.detail
	}
	return r
}

func checkConfigDir() checkResult {
	dir, err := config.GlobalDir()
	if err != nil {
		return checkResult{statusFail, "config dir", err.Error()}
	}
	return checkResult{statusOK, "config dir", dir}
}

func checkAuth() checkResult {
	store, err := auth.LoadStore()
	if err != nil {
		return checkResult{statusFail, "auth", err.Error()}
	}
	hosts := store.Hosts()
	if len(hosts) == 0 {
		return checkResult{statusWarn, "auth", "not logged in to any registry — run `zorvok login <email>` to publish with `zorvok trunk push`"}
	}
	// Hosts() iterates a map; sort so this line's order is stable
	// across runs instead of changing from one `zorvok doctor` call to
	// the next with no underlying change.
	sort.Strings(hosts)
	return checkResult{statusOK, "auth", fmt.Sprintf("logged in to: %s", strings.Join(hosts, ", "))}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
