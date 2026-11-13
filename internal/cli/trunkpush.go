package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/zorvok-dev/zorvok-ios/internal/publicregistry"
	"github.com/zorvok-dev/zorvok-ios/internal/publisher"
)

// runTrunkPush implements `zorvok trunk push [PODSPEC] [--allow-warnings]
// [--skip-lint]`, mirroring `pod trunk push`: lint locally, convert to
// JSON via `pod ipc spec`, and publish — to Zorvok's public registry
// (ADR 0010), unless registry_url names a Zorvok registry server, in
// which case to that server (ARCHITECTURE.md §2.4, brief §6).
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

	configured, explicit, err := registryURLExplicit()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	if !explicit || publicregistry.IsURL(configured) {
		return pushToPublicRegistry(out, errOut, cwd, opts)
	}

	client, url, token, err := loggedInClient()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	published, err := publisher.Push(context.Background(), out, errOut, cwd, client, token, opts)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	io.WriteString(out, "Published "+published.Package+" "+published.Version+" (checksum "+published.Checksum+")\n")
	io.WriteString(out, url+"/v1/packages/"+published.Package+"\n")
	return 0
}

// publishWaitTimeout bounds how long `trunk push` waits for the public
// registry to accept a podspec; a var so tests can shorten it.
var publishWaitTimeout = 10 * time.Minute

// githubAPIURL is GitHub's REST API, overridable via
// ZORVOK_GITHUB_API_URL so tests can stand a local fake in for it.
func githubAPIURL() (string, error) {
	if v := os.Getenv("ZORVOK_GITHUB_API_URL"); v != "" {
		return v, checkTokenURL("ZORVOK_GITHUB_API_URL", v)
	}
	return publicregistry.DefaultAPIURL, nil
}

// pushToPublicRegistry publishes to Zorvok's public registry in one
// command, the way `pod trunk push` publishes to CocoaPods: it returns
// once the registry has accepted the podspec, or said why it hasn't.
func pushToPublicRegistry(out, errOut io.Writer, dir string, opts publisher.Options) int {
	ctx := context.Background()
	apiURL, err := githubAPIURL()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	token, err := publicregistry.Token()
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	client := publicregistry.NewClient(apiURL, token)
	// Checked before linting, which can take minutes, so a missing or
	// expired GitHub login fails right away.
	user, err := client.User(ctx)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	p, err := publisher.Prepare(out, errOut, dir, opts)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	fmt.Fprintf(out, "-> Publishing %s %s to Zorvok's public registry as %s\n", p.Name, p.Version, user.Login)
	sub, err := client.Publish(ctx, user, publicregistry.Spec{Name: p.Name, Version: p.Version, JSON: p.JSON, Source: p.SourceSummary()})
	if err != nil {
		return exitErr(errOut, "%v", err)
	}
	fmt.Fprintf(out, "-> Waiting for the registry to accept it (usually about a minute): %s\n", sub.URL)
	verdict, err := client.WaitForVerdict(ctx, sub, publishWaitTimeout)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	switch verdict.Outcome {
	case publicregistry.Published:
		fmt.Fprintf(out, "Published %s %s to Zorvok's public registry. It can take up to 5 minutes to show up everywhere.\n", p.Name, p.Version)
		fmt.Fprintf(out, "Use it with: pod '%s', :source => '%s'\n", p.Name, publicregistry.URL)
		return 0
	case publicregistry.NeedsReview:
		fmt.Fprintf(out, "Submitted %s %s. It needs a one-time review by a registry maintainer before it goes live:\n%s\n%s\n", p.Name, p.Version, indentLines(verdict.Details), sub.URL)
		return 0
	case publicregistry.Rejected:
		return exitErr(errOut, "the registry didn't accept %s %s:\n%s\n%s", p.Name, p.Version, indentLines(verdict.Details), sub.URL)
	default:
		fmt.Fprintf(out, "Submitted %s %s, but the registry hasn't finished checking it yet. Follow it here: %s\n", p.Name, p.Version, sub.URL)
		return 0
	}
}

func indentLines(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
