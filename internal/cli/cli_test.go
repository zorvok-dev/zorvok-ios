package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(args ...string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	code = Execute(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestVersion(t *testing.T) {
	out, _, code := run("--version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(out, "zorvok dev") {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestHelpNoArgsIsExitOne(t *testing.T) {
	// A bare `zorvok` with no subcommand is a usage error, not a no-op success.
	_, errOut, code := run()
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "USAGE") {
		t.Fatalf("expected usage text, got %q", errOut)
	}
}

func TestExplicitHelpIsExitZero(t *testing.T) {
	out, _, code := run("--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "zorvok doctor") {
		t.Fatalf("expected help text to mention doctor, got %q", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, errOut, code := run("frobnicate")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, `unknown command "frobnicate"`) {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

// TestPlannedTwoWordCommandIsHonestNotSilent is the only remaining
// planned-command case — every single-word planned command (search,
// info, login, trunk push's single-word sibling install/update/lib
// lint) now has a real implementation; only the two-word `registry
// add`/`registry list` (Milestone 5) are still pending.
func TestPlannedTwoWordCommandIsHonestNotSilent(t *testing.T) {
	_, errOut, code := run("registry", "add")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "not implemented yet") {
		t.Fatalf("expected an explicit not-implemented message, got %q", errOut)
	}
}
