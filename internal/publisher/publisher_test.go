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
