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

// TestPlannedTwoWordCommandIsHonestNotSilent exercises the planned-
// command mechanism itself — every real command now has an
// implementation (plannedCommands is currently empty), so this passes
// a local fake entry straight into execute (the package's actual
// mutable state is never touched, so this is safe under `go test
// -parallel`/`-race` even if sibling tests in this file run
// concurrently). Whatever the next genuinely planned command turns
// out to be, this is the mechanism it will use.
func TestPlannedTwoWordCommandIsHonestNotSilent(t *testing.T) {
	planned := []plannedCommand{{"someday feature", "A hypothetical future command", "Milestone 99"}}

	var out, errOut bytes.Buffer
	code := execute([]string{"someday", "feature"}, &out, &errOut, planned)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "not implemented yet") {
		t.Fatalf("expected an explicit not-implemented message, got %q", errOut.String())
	}
}
