package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestBrandedOutputRewritesOnlyTheCompletionLine feeds CocoaPods-shaped
// output in awkward pieces (a write boundary inside "Pod installation")
// and checks that only the completion summary changes: the rest of its
// line, its color codes, and every other line come through byte for byte.
func TestBrandedOutputRewritesOnlyTheCompletionLine(t *testing.T) {
	cocoapods := "Analyzing dependencies\n" +
		"Pod installation guide: https://guides.cocoapods.org\n" +
		"\x1b[32mPod installation complete! There is 1 dependency from the Podfile and 1 total pod installed.\x1b[0m\n" +
		"\n[!] Please close any current Xcode sessions\n"
	want := "Analyzing dependencies\n" +
		"Pod installation guide: https://guides.cocoapods.org\n" +
		"\x1b[32mZorvok install complete! There is 1 dependency from the Podfile and 1 total pod installed.\x1b[0m\n" +
		"\n[!] Please close any current Xcode sessions\n"

	var out bytes.Buffer
	b := newBrandedOutput(&out, "Zorvok install complete!")
	split := strings.Index(cocoapods, "installation complete") - 3
	for _, piece := range []string{cocoapods[:split], cocoapods[split : split+5], cocoapods[split+5:]} {
		if n, err := b.Write([]byte(piece)); err != nil || n != len(piece) {
			t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(piece))
		}
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Fatalf("output:\n%q\nwant:\n%q", out.String(), want)
	}
}

func TestBrandedOutputFlushesALastLineWithoutANewline(t *testing.T) {
	var out bytes.Buffer
	b := newBrandedOutput(&out, "Zorvok install complete!")
	b.Write([]byte("first\nPod installation complete! no newline"))
	if out.String() != "first\n" {
		t.Fatalf("before Flush: %q, want only the complete line", out.String())
	}
	b.Flush()
	if out.String() != "first\nZorvok install complete! no newline" {
		t.Fatalf("after Flush: %q", out.String())
	}
}

func TestPodArgsKeepsColorsOnlyForATerminal(t *testing.T) {
	devNull, err := os.Open(os.DevNull) // a character device, like a terminal
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	cases := []struct {
		name  string
		out   any
		extra []string
		want  []string
	}{
		{"buffer", &bytes.Buffer{}, []string{"--repo-update"}, []string{"install", "--repo-update"}},
		{"terminal", devNull, []string{"--repo-update"}, []string{"install", "--repo-update", "--ansi"}},
		{"terminal, user chose --no-ansi", devNull, []string{"--no-ansi"}, []string{"install", "--no-ansi"}},
	}
	for _, c := range cases {
		var got []string
		switch w := c.out.(type) {
		case *bytes.Buffer:
			got = podArgs(w, "install", c.extra)
		case *os.File:
			got = podArgs(w, "install", c.extra)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: podArgs = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestInstallAndUpdateSayZorvokComplete(t *testing.T) {
	stubPod(t, `echo "Analyzing dependencies"; echo "Pod installation complete! There are 2 dependencies from the Podfile and 3 total pods installed."`)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Podfile"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := Install(&out, &errOut, dir, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if want := "Analyzing dependencies\nZorvok install complete! There are 2 dependencies from the Podfile and 3 total pods installed.\n"; out.String() != want {
		t.Fatalf("install output = %q, want %q", out.String(), want)
	}

	out.Reset()
	if err := Update(&out, &errOut, dir, nil); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !strings.Contains(out.String(), "Zorvok update complete! There are 2 dependencies") {
		t.Fatalf("update output = %q", out.String())
	}
}
