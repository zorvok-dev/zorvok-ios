package publisher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPodspecRequiresExactlyOne(t *testing.T) {
	dir := t.TempDir()
	if _, err := FindPodspec(dir); err == nil {
		t.Fatal("expected an error with zero podspecs")
	}

	os.WriteFile(filepath.Join(dir, "A.podspec"), nil, 0o644)
	name, err := FindPodspec(dir)
	if err != nil {
		t.Fatalf("FindPodspec: %v", err)
	}
	if name != "A.podspec" {
		t.Fatalf("got %q", name)
	}

	os.WriteFile(filepath.Join(dir, "B.podspec"), nil, 0o644)
	if _, err := FindPodspec(dir); err == nil {
		t.Fatal("expected an error with two podspecs")
	} else if !strings.Contains(err.Error(), "A.podspec") || !strings.Contains(err.Error(), "B.podspec") {
		t.Fatalf("expected the error to name both podspecs, got %v", err)
	}
}

// TestFindPodspecIgnoresDotfilesDirectoriesAndDanglingSymlinks proves
// the single-match search only ever considers a real, readable
// regular file named *.podspec — a stray dotfile, a directory that
// happens to match the name pattern, or a symlink pointing at
// nothing must never be silently accepted as "the" podspec, or
// counted as a second, ambiguous match.
func TestFindPodspecIgnoresDotfilesDirectoriesAndDanglingSymlinks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Real.podspec"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, ".Hidden.podspec"), nil, 0o644)
	os.Mkdir(filepath.Join(dir, "LooksLikeAFile.podspec"), 0o755)
	os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "Dangling.podspec"))

	name, err := FindPodspec(dir)
	if err != nil {
		t.Fatalf("FindPodspec: %v", err)
	}
	if name != "Real.podspec" {
		t.Fatalf("got %q, want Real.podspec (the only real, regular-file match)", name)
	}
}

// TestFindPodspecFollowsValidSymlinks proves the regular-file check
// uses Stat (follows symlinks), not Lstat — a symlink that genuinely
// resolves to a regular file is a legitimate way to point at a
// podspec and must still be found.
func TestFindPodspecFollowsValidSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-target.txt")
	os.WriteFile(real, []byte("fake podspec content"), 0o644)
	os.Symlink(real, filepath.Join(dir, "Linked.podspec"))

	name, err := FindPodspec(dir)
	if err != nil {
		t.Fatalf("FindPodspec: %v", err)
	}
	if name != "Linked.podspec" {
		t.Fatalf("got %q, want Linked.podspec", name)
	}
}

func TestParsePodspecRequiresNameAndVersion(t *testing.T) {
	if _, _, _, err := parsePodspec(`{"name":"Foo"}`); err == nil {
		t.Fatal("expected an error with no version")
	}
	if _, _, _, err := parsePodspec(`not json`); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	name, version, source, err := parsePodspec(`{"name":"Foo","version":"1.0.0","source":{"git":"https://example.com/foo.git"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Foo" || version != "1.0.0" || source["git"] != "https://example.com/foo.git" {
		t.Fatalf("unexpected parse result: %q %q %v", name, version, source)
	}
}

func TestValidateSourceIsFetchable(t *testing.T) {
	cases := []struct {
		name    string
		source  map[string]any
		wantErr bool
	}{
		{"git source ok", map[string]any{"git": "https://example.com/foo.git"}, false},
		{"http source ok", map[string]any{"http": "https://example.com/foo.zip"}, false},
		{"nil source rejected", nil, true},
		{"bare path rejected", map[string]any{"path": "."}, true},
		{"unrecognized key rejected", map[string]any{"carrier_pigeon": "yes"}, true},
		{"empty git value rejected", map[string]any{"git": ""}, true},
		{"whitespace-only git value rejected", map[string]any{"git": "   "}, true},
	}
	for _, c := range cases {
		err := validateSourceIsFetchable(c.source)
		if c.wantErr && err == nil {
			t.Errorf("%s: expected an error, got nil", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
		}
	}
}
