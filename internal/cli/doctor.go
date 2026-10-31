package cli

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
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
// reported as warnings, not failures, in Milestone 1: `zorvok install`
// doesn't exist yet, so their absence doesn't block anything a Milestone-1
// user can actually do — but surfacing them now means `zorvok doctor`
// already tells the truth about what Milestone 2 will need.
func runDoctor(out, errOut io.Writer, _ []string) int {
	var results []checkResult

	results = append(results, checkPlatform())
	results = append(results, checkTool("git", "git --version", "needed for git: dependencies (M2)", statusWarn))
	results = append(results, checkTool("ruby", "ruby -v", "needed to run `pod install` during Milestones 2-5 (ADR 0002)", statusWarn))
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

// checkTool verifies a binary is on PATH and reports its version output.
func checkTool(name, versionCmd, missingHint string, missingSeverity checkStatus) checkResult {
	path, err := exec.LookPath(name)
	if err != nil {
		return checkResult{missingSeverity, name, "not found on PATH — " + missingHint}
	}

	parts := strings.Fields(versionCmd)
	out, err := exec.Command(parts[0], parts[1:]...).Output()
	version := strings.TrimSpace(string(out))
	if err != nil || version == "" {
		return checkResult{statusWarn, name, path + " (found, but couldn't read version)"}
	}
	return checkResult{statusOK, name, firstLine(version) + "  (" + path + ")"}
}

func checkCocoaPods() checkResult {
	path, err := exec.LookPath("pod")
	if err != nil {
		return checkResult{statusWarn, "cocoapods", "`pod` not found on PATH — needed for `zorvok install` (M2); install with `gem install cocoapods`"}
	}
	out, err := exec.Command("pod", "--version").Output()
	version := strings.TrimSpace(string(out))
	if err != nil || version == "" {
		return checkResult{statusWarn, "cocoapods", path + " (found, but `pod --version` failed)"}
	}
	return checkResult{statusOK, "cocoapods", "v" + firstLine(version) + "  (" + path + ")"}
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
		return checkResult{statusWarn, "auth", "not logged in to any registry — `zorvok login` is not implemented yet (M4)"}
	}
	return checkResult{statusOK, "auth", fmt.Sprintf("logged in to: %s", strings.Join(hosts, ", "))}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
