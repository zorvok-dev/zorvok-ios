package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/installer"
)

// runInstall implements
// `zorvok install [--native [--source <url> ...] [--no-registry]] [pod-install-flags...]`.
//
// By default this delegates entirely to `pod install` (ADR 0002 Phase
// 1-2) — the default is unchanged by Milestone 6. --native opts in to
// Zorvok's own installer shim (ADR 0002 Phase 3): behaviorally
// identical with no --source flags (verified by a differential test,
// engines/cocoapods/installer), and able to resolve dependencies from
// additional spec sources — via --source, repeatable — that are not
// declared anywhere in the Podfile, without ever writing to it. This is
// opt-in, not default, per ADR 0002's "ships behind a flag... before
// becoming default."
//
// Automatic federation (ADR 0008): with --native and no explicit
// --source, the configured registry_url is automatically added as a
// source — but only if registry_url was actually set by the user
// (zorvok config set registry_url ...), never the bundled local-dev
// default nobody configured. --no-registry opts back out per invocation
// even when a registry is configured. Either way, ADR 0007's ambiguity
// refusal still applies unconditionally — automatic federation never
// bypasses it.
func runInstall(out, errOut io.Writer, args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		return exitErr(errOut, "determine working directory: %v", err)
	}

	native, sources, noRegistry, rest, err := parseInstallArgs(args)
	if err != nil {
		return exitErr(errOut, "%v", err)
	}

	if native {
		if len(rest) > 0 {
			return exitErr(errOut, "--native does not support %v — the native installer has no equivalent of these `pod install` flags yet; drop --native to use them", rest)
		}
		if len(sources) == 0 && !noRegistry {
			if url, explicit, err := registryURLExplicit(); err != nil {
				return exitErr(errOut, "%v", err)
			} else if explicit {
				sources = append(sources, url+"/cdn/")
			}
		}
		if err := installer.InstallNative(out, errOut, cwd, sources); err != nil {
			return reportDelegationError(errOut, err)
		}
		return 0
	}

	if err := installer.Install(out, errOut, cwd, rest); err != nil {
		return reportDelegationError(errOut, err)
	}
	return 0
}

// parseInstallArgs splits out --native, --source <url> (repeatable), and
// --no-registry; everything else passes through unchanged to the
// delegated `pod install` path. --source/--no-registry are only
// meaningful with --native (the delegated path has no mechanism to use
// them at all — that's the entire reason Milestone 6 exists, see
// ADR 0004) and are rejected otherwise, rather than silently ignored.
func parseInstallArgs(args []string) (native bool, sources []string, noRegistry bool, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--native":
			native = true
		case "--no-registry":
			noRegistry = true
		case "--source":
			if i+1 >= len(args) {
				return false, nil, false, nil, fmt.Errorf("--source requires a URL argument")
			}
			i++
			sources = append(sources, args[i])
		default:
			rest = append(rest, args[i])
		}
	}
	if !native && len(sources) > 0 {
		return false, nil, false, nil, fmt.Errorf("--source requires --native (the delegated `pod install` path has no mechanism to use extra sources — see ADR 0004)")
	}
	if !native && noRegistry {
		return false, nil, false, nil, fmt.Errorf("--no-registry requires --native (automatic registry federation only applies to the native installer — see ADR 0008)")
	}
	return native, sources, noRegistry, rest, nil
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
