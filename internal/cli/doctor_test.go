package cli

import (
	"sort"
	"strings"
	"testing"
)

// TestCheckToolSurfacesRealVersionError proves a failure running the
// version command is reported with its actual underlying error rather
// than a generic, uninformative "couldn't read version" — the
// distinction matters because the real error (permissions, a crashing
// binary, etc.) is exactly what a user running `zorvok doctor` to
// diagnose their environment needs to see.
func TestCheckToolSurfacesRealVersionError(t *testing.T) {
	r := checkTool("false", "false", "false", "unused", statusWarn)
	if r.status != statusWarn {
		t.Fatalf("status = %v, want statusWarn", r.status)
	}
	if !strings.Contains(r.detail, "exit status") {
		t.Fatalf("expected the underlying exec error in the detail, got %q", r.detail)
	}
}

// TestCheckCocoaPodsReusesCheckTool is a light smoke test that the
// dedup didn't change checkCocoaPods' displayed name or "v"-prefixed
// version formatting.
func TestCheckCocoaPodsReusesCheckTool(t *testing.T) {
	r := checkCocoaPods()
	if r.name != "cocoapods" {
		t.Fatalf("name = %q, want %q", r.name, "cocoapods")
	}
	if r.status == statusOK && !strings.HasPrefix(r.detail, "v") {
		t.Fatalf("expected a %q-prefixed version, got %q", "v", r.detail)
	}
}

// TestDoctorListsLoggedInHostsSorted proves `zorvok doctor`'s "logged
// in to: ..." line lists hosts in a fixed, alphabetical order rather
// than whatever order Go's map iteration happened to produce that run
// — otherwise the same, unchanged environment could print a different
// line from one `zorvok doctor` invocation to the next.
func TestDoctorListsLoggedInHostsSorted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	var ignored string
	srvA := fakeRegistrySrv(t, &ignored)
	srvB := fakeRegistrySrv(t, &ignored)

	hosts := []string{srvA.URL, srvB.URL}
	sort.Strings(hosts)

	for _, h := range hosts {
		run("config", "set", "registry_url", h, "--global")
		if _, errOut, code := run("login", "dev@example.com"); code != 0 {
			t.Fatalf("login to %s failed: %q", h, errOut)
		}
	}

	out, errOut, code := run("doctor")
	if code != 0 {
		t.Fatalf("doctor failed: code=%d stderr=%q", code, errOut)
	}

	want := "logged in to: " + strings.Join(hosts, ", ")
	if !strings.Contains(out, want) {
		t.Fatalf("expected %q in doctor output, got:\n%s", want, out)
	}
}
