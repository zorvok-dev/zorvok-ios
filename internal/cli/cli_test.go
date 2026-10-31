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

func TestPlannedCommandIsHonestNotSilent(t *testing.T) {
	// `search` is a real, named product command with no implementation yet.
	// It must say so loudly, never exit 0 as if it had done something.
	_, errOut, code := run("search")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "not implemented yet") {
		t.Fatalf("expected an explicit not-implemented message, got %q", errOut)
	}
}

func TestPlannedTwoWordCommandIsHonestNotSilent(t *testing.T) {
	_, errOut, code := run("registry", "add")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "not implemented yet") {
		t.Fatalf("expected an explicit not-implemented message, got %q", errOut)
	}
}
