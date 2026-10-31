package adapter

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// stubPod puts a fake `pod` shell script on PATH for the duration of the
// test, so these tests exercise our subprocess/exit-code handling without
// depending on a real CocoaPods install.
func stubPod(t *testing.T, script string) {
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

func TestFindNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir, guaranteed no `pod`
	if _, err := Find(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestRunStreamsOutputOnSuccess(t *testing.T) {
	stubPod(t, `echo "installing things"; exit 0`)

	var out, errOut bytes.Buffer
	if err := Run(&out, &errOut, t.TempDir(), "install"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.String() != "installing things\n" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestRunNonZeroExitIsCommandFailed(t *testing.T) {
	stubPod(t, `echo "boom" >&2; exit 3`)

	var out, errOut bytes.Buffer
	err := Run(&out, &errOut, t.TempDir(), "install")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("got %v, want ErrCommandFailed", err)
	}
	if errOut.String() != "boom\n" {
		t.Fatalf("expected pod's own stderr to pass through, got %q", errOut.String())
	}
}

func TestRunPassesWorkingDirectory(t *testing.T) {
	stubPod(t, `pwd`)

	dir := t.TempDir()

	var out, errOut bytes.Buffer
	if err := Run(&out, &errOut, dir, "install"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := out.String()
	if len(got) > 0 && got[len(got)-1] == '\n' {
		got = got[:len(got)-1]
	}
	if got != dir {
		t.Fatalf("working dir = %q, want %q", got, dir)
	}
}
