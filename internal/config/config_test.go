package config

import (
	"path/filepath"
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
