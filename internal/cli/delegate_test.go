package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubPod puts a fake `pod` executable on PATH that just echoes its own
// argv, so these tests verify Zorvok forwards the right arguments
// without needing a real CocoaPods install — the same pattern
// engines/cocoapods/repo's tests use.
func stubPod(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pod")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDelegatedCommandsForwardArgv(t *testing.T) {
	stubPod(t, `echo "$@"`)

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"registry", "lint", "myrepo", "--quick"}, "repo lint myrepo --quick\n"},
		{[]string{"registry", "push", "myrepo", "A.podspec"}, "repo push myrepo A.podspec\n"},
		{[]string{"lib", "create", "MyPod"}, "lib create MyPod\n"},
		{[]string{"spec", "lint", "A.podspec"}, "spec lint A.podspec\n"},
		{[]string{"spec", "cat", "Alamofire"}, "spec cat Alamofire\n"},
		{[]string{"cache", "clean", "--all"}, "cache clean --all\n"},
		{[]string{"cache", "list"}, "cache list\n"},
		{[]string{"ipc", "podfile-json", "Podfile"}, "ipc podfile-json Podfile\n"},
		{[]string{"ipc", "spec", "A.podspec"}, "ipc spec A.podspec\n"},
		{[]string{"plugins", "installed"}, "plugins installed\n"},
		{[]string{"plugins"}, "plugins\n"},
		{[]string{"list"}, "list\n"},
		{[]string{"outdated"}, "outdated\n"},
		{[]string{"try", "Alamofire"}, "try Alamofire\n"},
		{[]string{"env"}, "env\n"},
		{[]string{"setup"}, "setup\n"},
		{[]string{"deintegrate"}, "deintegrate\n"},
	}

	for _, c := range cases {
		out, errOut, code := run(c.args...)
		if code != 0 {
			t.Errorf("args %v: exit code = %d, stderr %q", c.args, code, errOut)
			continue
		}
		if out != c.want {
			t.Errorf("args %v: got argv %q, want %q", c.args, out, c.want)
		}
	}
}

func TestDelegatedCommandPropagatesPodFailure(t *testing.T) {
	stubPod(t, `echo "pod's own diagnostics" >&2; exit 1`)

	_, errOut, code := run("env")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// pod already streamed its own diagnostics to errOut — Zorvok must
	// not add a redundant "zorvok: pod command failed" wrapper on top.
	if errOut != "pod's own diagnostics\n" {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

func TestDelegatedCommandReportsMissingPod(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no `pod` on this PATH

	_, errOut, code := run("env")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "not found on PATH") {
		t.Fatalf("expected a not-found-on-PATH message, got %q", errOut)
	}
}
