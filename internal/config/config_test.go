package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Keys()) != 0 {
		t.Fatalf("expected empty config, got %v", cfg.Keys())
	}
}

func TestSetSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Set("email", "dev@example.com")
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	v, ok := reloaded.Get("email")
	if !ok || v != "dev@example.com" {
		t.Fatalf("got (%q, %v), want (\"dev@example.com\", true)", v, ok)
	}
}

func TestUnset(t *testing.T) {
	cfg, _ := Load(filepath.Join(t.TempDir(), "config.yml"))
	cfg.Set("key", "value")
	cfg.Unset("key")
	if _, ok := cfg.Get("key"); ok {
		t.Fatal("expected key to be unset")
	}
}

// TestLoadSkipsNonScalarValuesInsteadOfFailingEntireFile proves a
// single hand-edited key with a list/map value (where a scalar
// belongs) doesn't take the rest of the file down with it. Before
// this was fixed, yaml.Unmarshal into map[string]string failed on
// the very first non-scalar value, and Load returned that as a
// top-level error — every other, perfectly valid key in the file
// became unreachable too, for every zorvok command, until the user
// found and fixed the one bad line themselves.
func TestLoadSkipsNonScalarValuesInsteadOfFailingEntireFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(path, []byte("email: dev@example.com\ntags:\n  - a\n  - b\ndefault_registry: https://example.com\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok := cfg.Get("email"); !ok || v != "dev@example.com" {
		t.Fatalf("got (%q, %v), want (\"dev@example.com\", true)", v, ok)
	}
	if v, ok := cfg.Get("default_registry"); !ok || v != "https://example.com" {
		t.Fatalf("got (%q, %v), want (\"https://example.com\", true)", v, ok)
	}
	if _, ok := cfg.Get("tags"); ok {
		t.Fatal("non-scalar key should be dropped, not surfaced as a string")
	}
}

// TestGlobalDirTightensPermissionsOnAnExistingDirectory proves a
// pre-existing global config dir left at loose permissions (by an
// older zorvok version, or anything else) gets tightened to 0700,
// not just a freshly-created one. MkdirAll on its own is a no-op on
// a directory that already exists, so this directory — which holds
// credentials.yml — could otherwise stay world/group readable
// indefinitely.
func TestGlobalDirTightensPermissionsOnAnExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits don't apply on Windows")
	}
	base := t.TempDir()
	var dir string
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", base)
		dir = filepath.Join(base, "Library", "Application Support", "zorvok")
	} else {
		t.Setenv("XDG_CONFIG_HOME", base)
		dir = filepath.Join(base, "zorvok")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := GlobalDir()
	if err != nil {
		t.Fatalf("GlobalDir: %v", err)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("got permissions %o, want 0700", perm)
	}
}

func TestMergedProjectOverridesGlobal(t *testing.T) {
	global, _ := Load(filepath.Join(t.TempDir(), "g.yml"))
	global.Set("default_registry", "https://cocoapods.org")
	global.Set("telemetry", "on")

	project, _ := Load(filepath.Join(t.TempDir(), "p.yml"))
	project.Set("default_registry", "https://registry.zorvok.dev")

	merged := Merged(global, project)

	if v, _ := merged.Get("default_registry"); v != "https://registry.zorvok.dev" {
		t.Fatalf("project value should win, got %q", v)
	}
	if v, _ := merged.Get("telemetry"); v != "on" {
		t.Fatalf("global-only value should survive merge, got %q", v)
	}
}
