// Package publisher drives `zorvok trunk push`: find the podspec, lint
// it locally, convert it to JSON via `pod ipc spec`, and publish to the
// configured registry.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/podspec"
	"github.com/zorvok-dev/zorvok-ios/internal/registry"
)

// Options configures a single `trunk push` run.
type Options struct {
	// PodspecPath is the podspec to publish, relative to Dir. If empty,
	// FindPodspec locates the single *.podspec in Dir.
	PodspecPath string
	// SkipLint bypasses the local `pod lib lint` pass — real `pod trunk
	// push` supports this too (its own --skip-tests/--allow-warnings
	// flags trade thoroughness for speed on a known-good spec).
	SkipLint bool
	// AllowWarnings forwards --allow-warnings to the lint step.
	AllowWarnings bool
}

// FindPodspec locates the single *.podspec file in dir, matching `pod
// trunk push`'s own "no argument needed in a pod's own repo" ergonomics.
// More than one or zero podspecs is an error — Zorvok never guesses
// which one the user means.
//
// Lists the directory directly rather than filepath.Glob(dir +
// "/*.podspec"): a dir containing a glob metacharacter ([, ], *, ?)
// would otherwise have part of its own path misinterpreted as pattern
// syntax, not just the "*.podspec" suffix. Dotfiles are excluded
// (unlike Glob, which has no such convention) and only regular files
// (or symlinks to one) are considered, so a stray directory or a
// dangling symlink that happens to match the name pattern doesn't get
// handed to `pod lib lint`/`pod ipc spec` with a confusing low-level
// error.
func FindPodspec(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("searching for a podspec: %w", err)
	}
	var matches []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".podspec") {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.Mode().IsRegular() {
			continue
		}
		matches = append(matches, name)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no *.podspec file found in %s — pass one explicitly: zorvok trunk push MyPod.podspec", dir)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple podspecs found in %s (%s) — specify which one: zorvok trunk push <name>.podspec", dir, strings.Join(matches, ", "))
	}
}

// Push runs the full publish workflow and returns the published
// version's registry record.
func Push(ctx context.Context, out, errOut io.Writer, dir string, client *registry.Client, sessionToken string, opts Options) (*registry.PublishedVersion, error) {
	podspecPath := opts.PodspecPath
	if podspecPath == "" {
		found, err := FindPodspec(dir)
		if err != nil {
			return nil, err
		}
		podspecPath = found
	} else if info, err := os.Stat(filepath.Join(dir, podspecPath)); err != nil {
		return nil, fmt.Errorf("%s: %w", podspecPath, err)
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", podspecPath)
	}

	// ctx isn't passed to podspec.Lint/podspec.JSON below — neither
	// accepts one; they shell out via exec.Command, not
	// exec.CommandContext. A `pod lib lint` or `pod ipc spec` that
	// hangs can't be cancelled by ctx the way the PublishVersion call
	// further down can. Fixing that belongs in engines/cocoapods
	// alongside those functions, not here.
	if !opts.SkipLint {
		fmt.Fprintf(out, "-> Linting %s\n", podspecPath)
		lintArgs := []string{podspecPath}
		if opts.AllowWarnings {
			lintArgs = append(lintArgs, "--allow-warnings")
		}
		if err := podspec.Lint(out, errOut, dir, lintArgs); err != nil {
			return nil, fmt.Errorf("lint failed, not publishing: %w", err)
		}
	}

	fmt.Fprintf(out, "-> Reading %s\n", podspecPath)
	podspecJSON, err := podspec.JSON(errOut, dir, podspecPath)
	if err != nil {
		return nil, fmt.Errorf("could not read podspec: %w", err)
	}

	name, version, source, err := parsePodspec(podspecJSON)
	if err != nil {
		return nil, err
	}
	if err := validateSourceIsFetchable(source); err != nil {
		return nil, err
	}

	fmt.Fprintf(out, "-> Publishing %s %s\n", name, version)
	published, err := client.PublishVersion(ctx, sessionToken, name, version, podspecJSON, sourceSummary(source))
	if err != nil {
		return nil, err
	}
	return published, nil
}

func parsePodspec(podspecJSON string) (name, version string, source map[string]any, err error) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(podspecJSON), &decoded); err != nil {
		return "", "", nil, fmt.Errorf("podspec did not produce valid JSON: %w", err)
	}
	name, _ = decoded["name"].(string)
	version, _ = decoded["version"].(string)
	if name == "" || version == "" {
		return "", "", nil, fmt.Errorf("podspec is missing a name or version")
	}
	source, _ = decoded["source"].(map[string]any)
	return name, version, source, nil
}

// fetchableSourceKinds names the CocoaPods source types other
// machines can actually fetch from, reused across
// validateSourceIsFetchable's error messages below.
const fetchableSourceKinds = "a :git, :http, :svn, or :hg source"

// validateSourceIsFetchable rejects publishing a podspec whose `source`
// is a bare local :path — nobody else could install it, so a registry
// publish of it is always a mistake, not a real workflow. Real
// publishing needs a :git/:http/:svn/:hg source other machines can
// reach, matching CocoaPods' own source types.
func validateSourceIsFetchable(source map[string]any) error {
	if source == nil {
		return fmt.Errorf("podspec has no `source` — add %s others can fetch from", fetchableSourceKinds)
	}
	for _, key := range []string{"git", "http", "svn", "hg"} {
		if v, ok := source[key].(string); ok && strings.TrimSpace(v) != "" {
			return nil
		}
	}
	if _, ok := source["path"]; ok {
		return fmt.Errorf("podspec's source is a local :path — publish requires %s other users can fetch from", fetchableSourceKinds)
	}
	return fmt.Errorf("podspec's source has no recognized :git/:http/:svn/:hg key")
}

// sourceSummary re-encodes source, which always comes from
// parsePodspec's own json.Unmarshal result — a tree of only the
// types encoding/json produces for interface{} (map, slice, string,
// float64, bool, nil), none of which json.Marshal can fail on. The
// error is deliberately ignored, not swallowed by accident.
func sourceSummary(source map[string]any) string {
	b, _ := json.Marshal(source)
	return string(b)
}
