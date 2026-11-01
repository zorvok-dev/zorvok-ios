package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

func TestRedactToken(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"abc":          "***",
		"abcdef":       "******",
		"abcdefgh":     "abcdef…",
		"abcdefghijkl": "abcdef…",
	}
	for in, want := range cases {
		if got := RedactToken(in); got != want {
			t.Errorf("RedactToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStoreSetGetRemove(t *testing.T) {
	s := &Store{sessions: map[string]Credentials{}}

	if _, ok := s.Get("registry.zorvok.dev"); ok {
		t.Fatal("expected no credentials initially")
	}

	s.Set("registry.zorvok.dev", Credentials{Email: "dev@example.com", Token: "abcdef123456"})
	c, ok := s.Get("registry.zorvok.dev")
	if !ok || c.Email != "dev@example.com" {
		t.Fatalf("got (%+v, %v)", c, ok)
	}

	if hosts := s.Hosts(); len(hosts) != 1 || hosts[0] != "registry.zorvok.dev" {
		t.Fatalf("unexpected hosts: %v", hosts)
	}

	s.Remove("registry.zorvok.dev")
	if _, ok := s.Get("registry.zorvok.dev"); ok {
		t.Fatal("expected credentials to be removed")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "") // force the HOME-based default on every OS

	s, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	s.Set("registry.zorvok.dev", Credentials{Email: "dev@example.com", Token: "abcdef123456"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	c, ok := reloaded.Get("registry.zorvok.dev")
	if !ok || c.Email != "dev@example.com" || c.Token != "abcdef123456" {
		t.Fatalf("got (%+v, %v)", c, ok)
	}
}

// TestConcurrentSaveForDifferentHostsDoesNotLoseEither reproduces the
// real scenario Save's merge-before-write exists for: two separate
// `zorvok login` invocations (two independent Store instances, each
// loaded before the other's Save landed) targeting two different
// registries. A naive overwrite-with-my-snapshot Save would have the
// second Save erase the first's host entry entirely, even though
// neither process ever asked to remove it.
func TestConcurrentSaveForDifferentHostsDoesNotLoseEither(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	first, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadStore() // loaded before first's Save below
	if err != nil {
		t.Fatal(err)
	}

	first.Set("registry-a.example.com", Credentials{Email: "a@example.com", Token: "token-a"})
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}

	second.Set("registry-b.example.com", Credentials{Email: "b@example.com", Token: "token-b"})
	if err := second.Save(); err != nil {
		t.Fatal(err)
	}

	final, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Get("registry-a.example.com"); !ok {
		t.Error("registry-a's session was lost by the second, concurrent Save")
	}
	if _, ok := final.Get("registry-b.example.com"); !ok {
		t.Error("registry-b's session is missing")
	}
}

func TestRemovePersistsAcrossSave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	s, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	s.Set("keep.example.com", Credentials{Email: "keep@example.com", Token: "t1"})
	s.Set("drop.example.com", Credentials{Email: "drop@example.com", Token: "t2"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	reloaded.Remove("drop.example.com")
	if err := reloaded.Save(); err != nil {
		t.Fatal(err)
	}

	final, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Get("drop.example.com"); ok {
		t.Error("expected drop.example.com to be gone after Remove + Save")
	}
	if _, ok := final.Get("keep.example.com"); !ok {
		t.Error("expected keep.example.com to remain")
	}
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	s, err := LoadStore()
	if err != nil {
		t.Fatal(err)
	}
	s.Set("registry.zorvok.dev", Credentials{Email: "dev@example.com", Token: "abc123"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	dir, err := config.GlobalDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("Save left a temp file behind: %s", e.Name())
		}
	}
}
