package cli

import (
	"strings"
	"testing"
)

// TestTrunkDeprecateRejectsUnrecognizedFlag proves a typo'd flag
// (--in-favour-of, British spelling, instead of --in-favor-of) is
// reported as an error instead of being silently ignored — before this
// was fixed, the loop only ever matched the correct spelling and threw
// away anything else, so `zorvok trunk deprecate Foo --in-favour-of=Bar`
// would "succeed" with an empty deprecation message, never telling the
// caller their replacement pod was dropped.
func TestTrunkDeprecateRejectsUnrecognizedFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("trunk", "deprecate", "Alamofire", "--in-favour-of=Other")
	if code != 1 {
		t.Fatalf("code = %d, want 1 for a typo'd flag", code)
	}
	if !strings.Contains(errOut, `unrecognized argument "--in-favour-of=Other"`) {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

// TestTrunkDeprecateRejectsEmptyInFavorOfValue covers the other
// previously-silent case: the flag spelled correctly but with no value.
func TestTrunkDeprecateRejectsEmptyInFavorOfValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("trunk", "deprecate", "Alamofire", "--in-favor-of=")
	if code != 1 {
		t.Fatalf("code = %d, want 1 for an empty --in-favor-of value", code)
	}
	if !strings.Contains(errOut, "unrecognized argument") {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

// TestTrunkDeprecateAcceptsValidFlag confirms the stricter parsing
// above doesn't reject the legitimate spelling — it should get past
// argument validation and fail only on "not logged in" (there's no
// stored session in this fresh HOME), not on flag parsing.
func TestTrunkDeprecateAcceptsValidFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("trunk", "deprecate", "Alamofire", "--in-favor-of=Other")
	if code != 1 {
		t.Fatalf("code = %d, want 1 (not logged in)", code)
	}
	if strings.Contains(errOut, "unrecognized argument") {
		t.Fatalf("a correctly-spelled flag was rejected: %q", errOut)
	}
	if !strings.Contains(errOut, "not logged in") {
		t.Fatalf("expected a not-logged-in error, got %q", errOut)
	}
}

// TestTrunkDeleteMessageHasNoInternalCitation proves the user-facing
// message doesn't leak an internal document reference ("ADR 0003") —
// the kind of thing that reads as an AI-generated placeholder rather
// than a message a human wrote for someone else to read.
func TestTrunkDeleteMessageHasNoInternalCitation(t *testing.T) {
	_, errOut, _ := run("trunk", "delete", "Alamofire", "1.0.0")
	if strings.Contains(errOut, "ADR") {
		t.Fatalf("user-facing message cites an internal document: %q", errOut)
	}
	if !strings.Contains(errOut, "immutable") {
		t.Fatalf("expected the message to still explain why, got %q", errOut)
	}
}
