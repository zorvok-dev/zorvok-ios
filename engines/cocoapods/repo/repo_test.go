package repo

import (
	"bytes"
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

func TestListForwardsArgv(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := List(&out, &errOut, t.TempDir(), []string{"--count-only"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo list --count-only\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}

func TestAddForwardsNameURL(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := Add(&out, &errOut, t.TempDir(), "myrepo", "https://example.com/specs.git", nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo add myrepo https://example.com/specs.git\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}

func TestAddRequiresNameAndURL(t *testing.T) {
	stubPod(t, `exit 0`)
	var out, errOut bytes.Buffer
	if err := Add(&out, &errOut, t.TempDir(), "", "https://example.com", nil); err == nil {
		t.Fatal("expected an error with an empty name")
	}
	if err := Add(&out, &errOut, t.TempDir(), "name", "", nil); err == nil {
		t.Fatal("expected an error with an empty url")
	}
}

func TestAddCDNForwardsNameURL(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := AddCDN(&out, &errOut, t.TempDir(), "zorvok", "https://registry.zorvok.dev/cdn/", nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo add-cdn zorvok https://registry.zorvok.dev/cdn/\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}

func TestRemoveForwardsName(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := Remove(&out, &errOut, t.TempDir(), "myrepo", nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo remove myrepo\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}

func TestRemoveRequiresName(t *testing.T) {
	stubPod(t, `exit 0`)
	var out, errOut bytes.Buffer
	if err := Remove(&out, &errOut, t.TempDir(), "", nil); err == nil {
		t.Fatal("expected an error with an empty name")
	}
}

func TestUpdateWithName(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := Update(&out, &errOut, t.TempDir(), "myrepo", nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo update myrepo\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}

func TestUpdateWithoutNameUpdatesAll(t *testing.T) {
	stubPod(t, `echo "$@"`)
	var out, errOut bytes.Buffer
	if err := Update(&out, &errOut, t.TempDir(), "", nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo update\n" {
		t.Fatalf("unexpected argv: %q", got)
	}
}
