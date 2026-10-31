// Package compatibility holds Zorvok's CocoaPods compatibility test
// suite (ARCHITECTURE.md §6). Per the roadmap, Zorvok does not claim
// CocoaPods compatibility anywhere until these tests are green.
//
// These tests exercise `zorvok install` against real `pod install` — no
// mocking of CocoaPods itself — using a local-path dependency fixture
// specifically so they run hermetically, with no network access and no
// real Xcode.app build required (only the `xcodeproj` gem, a CocoaPods
// dependency, to read/write the project file). They skip, rather than
// fail, when no local CocoaPods install is available, since Milestone 2
// still depends on one per ADR 0002.
package compatibility

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/installer"
)

const fixturesDir = "../../../tests/fixtures"

func requirePod(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found (CocoaPods is a Milestone 2 runtime dependency, see ADR 0002)")
	}
}

// copyFixture copies a fixture directory tree into dst so the test never
// writes into the checked-in tests/fixtures/ directory itself.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatalf("copy fixture %s -> %s: %v", src, dst, err)
	}
}

// TestSimpleLocalPathDependency is the Milestone 2 smoke test for the
// acceptance criteria in ARCHITECTURE.md §6: run `zorvok install`
// against an existing CocoaPods project and verify it produces a working
// installation, unmodified Podfile, and a reproducible lockfile.
func TestSimpleLocalPathDependency(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyFixture(t, filepath.Join(fixturesDir, "local-pod"), filepath.Join(root, "local-pod"))
	copyFixture(t, filepath.Join(fixturesDir, "simple-project"), filepath.Join(root, "simple-project"))

	projectDir := filepath.Join(root, "simple-project")
	originalPodfile, err := os.ReadFile(filepath.Join(projectDir, "Podfile"))
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	if err := installer.Install(&out, &errOut, projectDir, nil); err != nil {
		t.Fatalf("install failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	// The Podfile itself must never be touched — Zorvok's entire premise.
	podfileAfter, err := os.ReadFile(filepath.Join(projectDir, "Podfile"))
	if err != nil {
		t.Fatal(err)
	}
	if string(originalPodfile) != string(podfileAfter) {
		t.Fatal("Podfile was modified by install — Zorvok must never rewrite an existing Podfile")
	}

	lock, err := os.ReadFile(filepath.Join(projectDir, "Podfile.lock"))
	if err != nil {
		t.Fatalf("Podfile.lock was not created: %v", err)
	}
	lockStr := string(lock)
	for _, want := range []string{"PODS:", "LocalPod (0.1.0)", "SPEC CHECKSUMS:", "PODFILE CHECKSUM:"} {
		if !strings.Contains(lockStr, want) {
			t.Errorf("Podfile.lock missing expected section %q:\n%s", want, lockStr)
		}
	}

	for _, path := range []string{
		filepath.Join(projectDir, "Pods"),
		filepath.Join(projectDir, "Pods", "Pods.xcodeproj"),
		filepath.Join(projectDir, "SimpleApp.xcworkspace"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist after install: %v", path, err)
		}
	}
}
