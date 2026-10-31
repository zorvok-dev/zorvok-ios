package adapter

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// RubyEnv is the Ruby interpreter + optional GEM_HOME needed to run code
// against the same CocoaPods gem install the local `pod` executable
// uses. Milestone 6 (ADR 0002 Phase 3) needs this to run Zorvok's own
// Ruby shim (installer/zorvok_install.rb) against `cocoapods-core` +
// `molinillo` + `xcodeproj` directly, bypassing the `pod` CLI binary —
// but it still has to find and load the exact same gems `pod` itself
// uses, on whatever machine it's running on.
type RubyEnv struct {
	// RubyPath is the Ruby interpreter to invoke.
	RubyPath string
	// GemHome, if non-empty, must be set as the GEM_HOME environment
	// variable for RubyPath to find the `cocoapods` gem.
	GemHome string
}

var shebangRe = regexp.MustCompile(`^#!\s*(\S+)`)

// gemHomeExecRe matches Homebrew's generated wrapper shape:
//
//	GEM_HOME="/path/to/libexec" exec "/path/to/libexec/bin/pod" "$@"
//
// This is the common case in practice: Homebrew installs CocoaPods into
// an isolated Cellar path outside Ruby's default gem search path, so
// its `pod` executable is a small bash script that sets GEM_HOME before
// handing off to the real, RubyGems-generated `pod` script. A plain
// `gem install cocoapods` doesn't need this — its `pod` executable is
// already the RubyGems-generated script directly, found via RubyGems'
// own default resolution with no GEM_HOME override needed.
var gemHomeExecRe = regexp.MustCompile(`GEM_HOME="([^"]+)"\s+exec\s+"([^"]+)"`)

// ResolveRubyEnv inspects the `pod` executable `pod` is invoked at
// (Find()'s result) to determine how to run our own Ruby code against
// the same gem environment. It follows at most one layer of
// wrapper-script indirection (Homebrew's shape); anything more exotic
// falls back to whatever `ruby` is on PATH, which is a reasonable
// last resort, not a silent wrong answer — doctor/trunk push's error
// messages surface the failure if that fallback can't load cocoapods.
func ResolveRubyEnv() (RubyEnv, error) {
	podPath, err := Find()
	if err != nil {
		return RubyEnv{}, err
	}
	return resolveFromPodScript(podPath)
}

func resolveFromPodScript(podPath string) (RubyEnv, error) {
	shebang, err := readShebang(podPath)
	if err != nil {
		return RubyEnv{RubyPath: "ruby"}, nil // can't introspect; best-effort fallback
	}

	// Case 1: a direct Ruby script (`#!/usr/bin/ruby`, `#!/usr/bin/env ruby`, ...).
	// This is the `gem install cocoapods` shape — no GEM_HOME override needed.
	if shebang != "" && !strings.HasSuffix(shebang, "bash") && !strings.HasSuffix(shebang, "sh") {
		return RubyEnv{RubyPath: shebang}, nil
	}

	// Case 2: a bash wrapper. Read the rest of the file looking for the
	// `GEM_HOME="..." exec "..."` pattern (Homebrew's shape).
	data, err := os.ReadFile(podPath)
	if err != nil {
		return RubyEnv{RubyPath: "ruby"}, nil
	}
	m := gemHomeExecRe.FindStringSubmatch(string(data))
	if m == nil {
		// A bash wrapper we don't recognize; best effort.
		return RubyEnv{RubyPath: "ruby"}, nil
	}
	gemHome, realScript := m[1], m[2]

	realShebang, err := readShebang(realScript)
	if err != nil || realShebang == "" {
		return RubyEnv{RubyPath: "ruby", GemHome: gemHome}, nil
	}
	return RubyEnv{RubyPath: realShebang, GemHome: gemHome}, nil
}

func readShebang(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", fmt.Errorf("empty file")
	}
	m := shebangRe.FindStringSubmatch(scanner.Text())
	if m == nil {
		return "", nil
	}
	return m[1], nil
}
