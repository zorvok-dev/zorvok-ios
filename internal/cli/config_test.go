package cli

import (
	"strings"
	"testing"
)

func TestConfigSetGetProjectScoped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	if _, _, code := run("config", "set", "email", "dev@example.com"); code != 0 {
		t.Fatalf("set failed")
	}
	out, _, code := run("config", "get", "email")
	if code != 0 || strings.TrimSpace(out) != "dev@example.com" {
		t.Fatalf("got (%q, %d)", out, code)
	}
}

func TestConfigGetMissingKeyFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("config", "get", "nope")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "no value set") {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

func TestConfigGlobalPersistsAcrossProjects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	t.Chdir(t.TempDir())
	run("config", "set", "default_registry", "https://registry.zorvok.dev", "--global")

	t.Chdir(t.TempDir()) // a different project directory
	out, _, code := run("config", "get", "default_registry")
	if code != 0 || strings.TrimSpace(out) != "https://registry.zorvok.dev" {
		t.Fatalf("expected global value visible from another project dir, got (%q, %d)", out, code)
	}
}

func TestConfigUnset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	run("config", "set", "email", "dev@example.com")
	if _, _, code := run("config", "unset", "email"); code != 0 {
		t.Fatalf("unset failed")
	}
	_, _, code := run("config", "get", "email")
	if code != 1 {
		t.Fatalf("expected key to be gone after unset")
	}
}
