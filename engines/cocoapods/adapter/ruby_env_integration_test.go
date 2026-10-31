package adapter

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRunRubyScriptCanLoadRealCocoaPods is a real, non-mocked sanity
// check: it resolves this machine's actual `pod` installation and runs
// a one-line script through RunRubyScript that `require`s the
// `cocoapods` gem — if GEM_HOME/ruby-path resolution were wrong, this
// would fail exactly the way the manual investigation that led to
// ADR 0005 initially did (a bare `ruby -e "require 'cocoapods'"` with
// the wrong Ruby/GEM_HOME fails with a confusing Gem::MissingSpecError).
func TestRunRubyScriptCanLoadRealCocoaPods(t *testing.T) {
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found")
	}

	var out, errOut strings.Builder
	script := `require 'cocoapods'
puts "ok #{Pod::VERSION}"
`
	err := RunRubyScript(&out, &errOut, t.TempDir(), script)
	if err != nil {
		t.Fatalf("RunRubyScript: %v\nstderr: %s", err, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "ok ") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
