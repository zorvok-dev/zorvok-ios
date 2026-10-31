// Package publisher orchestrates `zorvok trunk push`: locate a podspec,
// lint it locally (client-side only — the registry server never
// executes package code, ARCHITECTURE.md §2.4), convert it to the
// declarative JSON form via `pod ipc spec`, and publish that to a
// Zorvok registry. Mirrors the real `pod trunk push` workflow's shape
// (brief §6) without reusing CocoaPods Trunk's implementation.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/podspec"
	"github.com/zorvok-dev/zorvok/internal/registry"
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
func FindPodspec(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.podspec"))
	if err != nil {
		return "", fmt.Errorf("searching for a podspec: %w", err)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no *.podspec file found in %s — pass one explicitly: zorvok trunk push MyPod.podspec", dir)
	case 1:
		return filepath.Base(matches[0]), nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = filepath.Base(m)
		}
		return "", fmt.Errorf("multiple podspecs found in %s (%s) — specify which one: zorvok trunk push <name>.podspec", dir, strings.Join(names, ", "))
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
	} else if _, err := os.Stat(filepath.Join(dir, podspecPath)); err != nil {
		return nil, fmt.Errorf("%s: %w", podspecPath, err)
	}

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

// validateSourceIsFetchable rejects publishing a podspec whose `source`
// is a bare local :path — nobody else could install it, so a registry
// publish of it is always a mistake, not a real workflow. Real
// publishing needs a :git/:http/:svn/:hg source other machines can
// reach, matching CocoaPods' own source types.
func validateSourceIsFetchable(source map[string]any) error {
	if source == nil {
		return fmt.Errorf("podspec has no `source` — add a :git, :http, :svn, or :hg source others can fetch from")
	}
	for _, key := range []string{"git", "http", "svn", "hg"} {
		if _, ok := source[key]; ok {
			return nil
		}
	}
	if _, ok := source["path"]; ok {
		return fmt.Errorf("podspec's source is a local :path — publish requires a :git, :http, :svn, or :hg source other users can fetch from")
	}
	return fmt.Errorf("podspec's source has no recognized :git/:http/:svn/:hg key")
}

func sourceSummary(source map[string]any) string {
	b, err := json.Marshal(source)
	if err != nil {
		return ""
	}
	return string(b)
}
