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

func TestConfigListSubcommandWorks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	run("config", "set", "email", "dev@example.com")
	// The doc comment has always advertised `zorvok config list` as a
	// valid spelling, same as bare `zorvok config` — it must actually
	// work, not fail with "unknown config subcommand".
	out, _, code := run("config", "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "email = dev@example.com") {
		t.Fatalf("expected email in `config list` output, got %q", out)
	}
}

func TestConfigGetAndListRespectGlobalFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	// Different values at each scope — global must win (visibly) only
	// when --global is passed, not be silently ignored in favor of the
	// merged (here, project-overrides-global) view.
	run("config", "set", "email", "global@example.com", "--global")
	run("config", "set", "email", "project@example.com")

	out, _, code := run("config", "get", "email", "--global")
	if code != 0 || strings.TrimSpace(out) != "global@example.com" {
		t.Fatalf("config get --global: got (%q, %d), want global@example.com", out, code)
	}

	listOut, _, code := run("config", "list", "--global")
	if code != 0 {
		t.Fatalf("config list --global: exit code = %d", code)
	}
	if !strings.Contains(listOut, "global@example.com") || strings.Contains(listOut, "project@example.com") {
		t.Fatalf("config list --global must show only the global file's values, got %q", listOut)
	}

	// Unflagged reads are unaffected: still the merged, project-wins view.
	out, _, code = run("config", "get", "email")
	if code != 0 || strings.TrimSpace(out) != "project@example.com" {
		t.Fatalf("config get (no flag): got (%q, %d), want project@example.com", out, code)
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
