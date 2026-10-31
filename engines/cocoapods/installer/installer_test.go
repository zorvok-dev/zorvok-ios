package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func stubPod(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pod")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInstallRequiresPodfile(t *testing.T) {
	stubPod(t, `exit 0`)

	var out, errOut bytes.Buffer
	err := Install(&out, &errOut, t.TempDir(), nil)
	if !errors.Is(err, ErrNoPodfile) {
		t.Fatalf("got %v, want ErrNoPodfile", err)
	}
}

func TestInstallForwardsArgsToPod(t *testing.T) {
	// Echo argv back so we can assert `install --repo-update` was the
	// exact command line passed through to `pod`.
	stubPod(t, `echo "$@"`)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Podfile"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := Install(&out, &errOut, dir, []string{"--repo-update"}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := out.String(); got != "install --repo-update\n" {
		t.Fatalf("unexpected argv passed to pod: %q", got)
	}
}

func TestUpdateRequiresPodfile(t *testing.T) {
	stubPod(t, `exit 0`)

	var out, errOut bytes.Buffer
	err := Update(&out, &errOut, t.TempDir(), nil)
	if !errors.Is(err, ErrNoPodfile) {
		t.Fatalf("got %v, want ErrNoPodfile", err)
	}
}

func TestUpdateForwardsPodNames(t *testing.T) {
	stubPod(t, `echo "$@"`)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Podfile"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := Update(&out, &errOut, dir, []string{"Alamofire", "SDWebImage"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := out.String(); got != "update Alamofire SDWebImage\n" {
		t.Fatalf("unexpected argv passed to pod: %q", got)
	}
}
