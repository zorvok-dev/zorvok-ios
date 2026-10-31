package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesProjectConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	out, _, code := run("init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("unexpected output: %q", out)
	}
	if _, err := os.Stat(".zorvok.yml"); err != nil {
		t.Fatalf(".zorvok.yml was not created: %v", err)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	run("init")
	out, _, code := run("init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "already initialized") {
		t.Fatalf("expected idempotent message, got %q", out)
	}
}

func TestInitDetectsPodfileWithoutModifyingIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)

	podfile := filepath.Join(dir, "Podfile")
	if err := os.WriteFile(podfile, []byte("platform :ios, '13.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(podfile)

	out, _, code := run("init")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out, "detected an existing Podfile") {
		t.Fatalf("expected Podfile detection, got %q", out)
	}

	after, _ := os.ReadFile(podfile)
	if string(before) != string(after) {
		t.Fatal("zorvok init must never modify an existing Podfile")
	}
}
