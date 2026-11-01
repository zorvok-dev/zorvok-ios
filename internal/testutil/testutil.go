// Package testutil holds small test helpers shared across
// engines/cocoapods's subpackages (adapter, installer, repo, podspec),
// which all need to exercise code that shells out to `pod` without
// depending on a real CocoaPods install being present.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// StubPod puts a fake `pod` shell script on PATH for the duration of the
// test, so callers can exercise subprocess/exit-code handling without
// depending on a real CocoaPods install.
func StubPod(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub not supported on windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "pod")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
