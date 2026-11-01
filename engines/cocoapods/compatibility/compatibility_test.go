// Package compatibility runs `zorvok install` against real `pod
// install` — no mocks — using hermetic fixtures (local-path and
// file:// git deps, no network, no Xcode.app). Tests skip rather than
// fail if CocoaPods/git aren't installed locally.
//
// Doesn't yet cover importing someone else's hand-written
// Podfile.lock; see ROADMAP.md.
package compatibility

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/installer"
)

const fixturesDir = "../../../tests/fixtures"

func requirePod(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found (CocoaPods is a Milestone 2 runtime dependency, see ADR 0002)")
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping: no local `git` install found")
	}
}

// copyTree copies the directory tree at src into dst.
func copyTree(t *testing.T, src, dst string) {
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

// copyRel copies tests/fixtures/<rel> into root/<rel>, preserving the
// fixture's relative path so that Podfiles using `:path => '../sibling'`
// still resolve correctly once copied out of the repo.
func copyRel(t *testing.T, root, rel string) {
	t.Helper()
	copyTree(t, filepath.Join(fixturesDir, rel), filepath.Join(root, rel))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func mustInstall(t *testing.T, dir string, extraArgs []string) (stdout, stderr string) {
	t.Helper()
	var out, errOut strings.Builder
	if err := installer.Install(&out, &errOut, dir, extraArgs); err != nil {
		t.Fatalf("install failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	return out.String(), errOut.String()
}

// TestSimpleLocalPathDependency is the Milestone 2 smoke test for the
// acceptance criteria in ARCHITECTURE.md §6: run `zorvok install`
// against an existing CocoaPods project and verify it produces a working
// installation, unmodified Podfile, and a reproducible lockfile.
func TestSimpleLocalPathDependency(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "local-pod")
	copyRel(t, root, "simple-project")
	projectDir := filepath.Join(root, "simple-project")

	originalPodfile := readFile(t, filepath.Join(projectDir, "Podfile"))
	mustInstall(t, projectDir, nil)

	// The Podfile itself must never be touched — Zorvok's entire premise.
	if got := readFile(t, filepath.Join(projectDir, "Podfile")); got != originalPodfile {
		t.Fatal("Podfile was modified by install — Zorvok must never rewrite an existing Podfile")
	}

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
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

// TestReinstallIsIdempotent re-runs install against an already-installed
// project and checks the lockfile doesn't drift — a prerequisite for
// trusting an existing, committed Podfile.lock the way real CocoaPods
// projects do.
func TestReinstallIsIdempotent(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "local-pod")
	copyRel(t, root, "simple-project")
	projectDir := filepath.Join(root, "simple-project")

	mustInstall(t, projectDir, nil)
	firstLock := readFile(t, filepath.Join(projectDir, "Podfile.lock"))

	mustInstall(t, projectDir, nil)
	secondLock := readFile(t, filepath.Join(projectDir, "Podfile.lock"))

	if firstLock != secondLock {
		t.Fatalf("Podfile.lock changed on a reinstall with nothing changed:\n--- first ---\n%s\n--- second ---\n%s", firstLock, secondLock)
	}
}

// TestTransitiveDependencyResolves: the Podfile declares PodB only, and
// PodB's own podspec depends on PodA; both must end up installed and
// recorded in the lockfile's dependency graph.
func TestTransitiveDependencyResolves(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "transitive/pod-a")
	copyRel(t, root, "transitive/pod-b")
	copyRel(t, root, "transitive/project")
	projectDir := filepath.Join(root, "transitive", "project")

	mustInstall(t, projectDir, nil)

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	for _, want := range []string{"PodA (0.1.0)", "PodB (0.1.0)"} {
		if !strings.Contains(lockStr, want) {
			t.Errorf("Podfile.lock missing %q:\n%s", want, lockStr)
		}
	}
}

// TestTransitiveDependencyMissingPathFails proves resolution is real,
// not just file-copying: a Podfile that omits a local :path dependency
// PodB actually needs must fail to install, with a clear diagnostic.
func TestTransitiveDependencyMissingPathFails(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "transitive/pod-a")
	copyRel(t, root, "transitive/pod-b")
	copyRel(t, root, "transitive/project-missing-dependency")
	projectDir := filepath.Join(root, "transitive", "project-missing-dependency")

	var out, errOut strings.Builder
	err := installer.Install(&out, &errOut, projectDir, nil)
	if err == nil {
		t.Fatal("expected install to fail when a transitive local dependency's path is undeclared")
	}
	if _, lockErr := os.Stat(filepath.Join(projectDir, "Podfile.lock")); lockErr == nil {
		t.Error("Podfile.lock should not be written for a failed resolution")
	}
}

// TestSubspecInstallsOnlyDeclaredSubspec: the Podfile declares
// `SubspecPod/Core` only; `Extra`'s source files must never appear in
// the generated Pods project.
func TestSubspecInstallsOnlyDeclaredSubspec(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "subspecs/pod")
	copyRel(t, root, "subspecs/project")
	projectDir := filepath.Join(root, "subspecs", "project")

	mustInstall(t, projectDir, nil)

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if !strings.Contains(lockStr, "SubspecPod/Core (0.1.0)") {
		t.Errorf("Podfile.lock missing SubspecPod/Core:\n%s", lockStr)
	}
	if strings.Contains(lockStr, "SubspecPod/Extra") {
		t.Errorf("Podfile.lock should not mention the undeclared Extra subspec:\n%s", lockStr)
	}

	pbxproj := readFile(t, filepath.Join(projectDir, "Pods", "Pods.xcodeproj", "project.pbxproj"))
	if !strings.Contains(pbxproj, "SubspecPodCore") {
		t.Error("expected the Core subspec's source file to be in the generated Pods project")
	}
	if strings.Contains(pbxproj, "SubspecPodExtra") {
		t.Error("the undeclared Extra subspec's source file must not be in the generated Pods project")
	}
}

// TestResourceBundleGeneratesBundleTarget verifies a podspec's
// `resource_bundles` produces a real bundle target in the generated Pods
// project — not just that install exits zero.
func TestResourceBundleGeneratesBundleTarget(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "resource-bundle/pod")
	copyRel(t, root, "resource-bundle/project")
	projectDir := filepath.Join(root, "resource-bundle", "project")

	mustInstall(t, projectDir, nil)

	pbxproj := readFile(t, filepath.Join(projectDir, "Pods", "Pods.xcodeproj", "project.pbxproj"))
	if !strings.Contains(pbxproj, `"com.apple.product-type.bundle"`) {
		t.Error("expected a com.apple.product-type.bundle target for the declared resource bundle")
	}
	if !strings.Contains(pbxproj, "ResourceBundlePod-ResourceBundlePod") {
		t.Error("expected the resource bundle target to be named after the pod and bundle")
	}
}

// TestMultiTargetInstallsPerTargetPods: two targets in one Podfile, with
// overlapping and distinct pods, must each get their own
// "Pods-<Target>" integration and the union of pods in the lockfile.
func TestMultiTargetInstallsPerTargetPods(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "transitive/pod-a")
	copyRel(t, root, "transitive/pod-b")
	copyRel(t, root, "multi-target/project")
	projectDir := filepath.Join(root, "multi-target", "project")

	mustInstall(t, projectDir, nil)

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	for _, want := range []string{"PodA (0.1.0)", "PodB (0.1.0)"} {
		if !strings.Contains(lockStr, want) {
			t.Errorf("Podfile.lock missing %q:\n%s", want, lockStr)
		}
	}

	for _, name := range []string{"Pods-AppOne", "Pods-AppTwo"} {
		path := filepath.Join(projectDir, "Pods", "Target Support Files", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected per-target support files at %s: %v", path, err)
		}
	}
}

// TestGitDependencyResolves verifies a `:git`/`:tag` dependency, the
// most common non-:path CocoaPods source, installs correctly. The git
// repo is generated on disk for this test run only (via a local file://
// URL) so the test needs no network access.
func TestGitDependencyResolves(t *testing.T) {
	requirePod(t)
	requireGit(t)

	root := t.TempDir()
	copyRel(t, root, "git-dependency/git-pod-source")
	copyRel(t, root, "git-dependency/project")

	repoDir := filepath.Join(root, "git-dependency", "git-pod-source")
	repoURL := "file://" + repoDir

	podspecPath := filepath.Join(repoDir, "GitPod.podspec")
	podspec := strings.ReplaceAll(readFile(t, podspecPath), "ZORVOK_GIT_REPO_URL", repoURL)
	if err := os.WriteFile(podspecPath, []byte(podspec), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=zorvok-test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=zorvok-test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q", "-b", "main")
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "init")
	runGit("tag", "1.0.0")

	projectDir := filepath.Join(root, "git-dependency", "project")
	podfileTemplatePath := filepath.Join(projectDir, "Podfile.template")
	podfile := strings.ReplaceAll(readFile(t, podfileTemplatePath), "ZORVOK_GIT_REPO_URL", repoURL)
	if err := os.WriteFile(filepath.Join(projectDir, "Podfile"), []byte(podfile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(podfileTemplatePath); err != nil {
		t.Fatal(err)
	}

	mustInstall(t, projectDir, nil)

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if !strings.Contains(lockStr, "GitPod (1.0.0)") {
		t.Errorf("Podfile.lock missing GitPod:\n%s", lockStr)
	}
	if !strings.Contains(lockStr, repoURL) {
		t.Errorf("Podfile.lock should record the git source:\n%s", lockStr)
	}
}

// TestXCFrameworkDependencyResolves verifies a pod distributed as a
// prebuilt binary (`s.vendored_frameworks`, an .xcframework) rather than
// source installs correctly and gets properly wired for linking — not
// just that install exits zero. The fixture's .xcframework was built
// with clang + `xcodebuild -create-xcframework` (see
// tests/fixtures/xcframework/README.md); `pod install` only needs to
// read its structure, so the committed binary is small and portable
// across machines/architectures for this test's purposes (it is never
// executed or compiled against here).
func TestXCFrameworkDependencyResolves(t *testing.T) {
	requirePod(t)

	root := t.TempDir()
	copyRel(t, root, "xcframework/pod")
	copyRel(t, root, "xcframework/project")
	projectDir := filepath.Join(root, "xcframework", "project")

	mustInstall(t, projectDir, nil)

	lockStr := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if !strings.Contains(lockStr, "XCFrameworkPod (0.1.0)") {
		t.Errorf("Podfile.lock missing XCFrameworkPod:\n%s", lockStr)
	}

	pbxproj := readFile(t, filepath.Join(projectDir, "Pods", "Pods.xcodeproj", "project.pbxproj"))
	if !strings.Contains(pbxproj, "ZorvokFixture") {
		t.Error("expected the vendored xcframework's binary name to appear in the generated Pods project")
	}

	appProj := readFile(t, filepath.Join(projectDir, "XCFrameworkApp.xcodeproj", "project.pbxproj"))
	if !strings.Contains(appProj, "PODS_XCFRAMEWORKS_BUILD_DIR") {
		t.Error("expected the host app target to be wired to link against the xcframework via PODS_XCFRAMEWORKS_BUILD_DIR")
	}
}

// TestUpdateMovesForwardButInstallStaysLocked is `zorvok update`'s own
// real compatibility test — until now it only had stub-based unit tests
// (engines/cocoapods/installer), never verified end-to-end against real
// CocoaPods like every other command here. The scenario that actually
// distinguishes `update` from `install`: a branch-tracking git
// dependency (no :tag, so there's a real "newer commit" for update to
// move to). Confirmed empirically before writing this test: plain
// `pod install` never re-resolves a branch-tracking git source once
// locked — only `pod update` does.
func TestUpdateMovesForwardButInstallStaysLocked(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()
	podGitDir := filepath.Join(root, "updatable-pod-git")
	copyRel(t, root, "updatable-pod")
	if err := os.Rename(filepath.Join(root, "updatable-pod"), podGitDir); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = podGitDir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=zorvok-test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=zorvok-test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	podspecPath := filepath.Join(podGitDir, "UpdatablePod.podspec")
	content := readFile(t, podspecPath)
	augmented := strings.ReplaceAll(content, "ZORVOK_UPDATABLE_POD_GIT_URL", "file://"+podGitDir)
	if err := os.WriteFile(podspecPath, []byte(augmented), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("init", "-q", "-b", "main")
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "v1.0.0")

	copyRel(t, root, "updatable-project")
	projectDir := filepath.Join(root, "updatable-project")
	template := readFile(t, filepath.Join(projectDir, "Podfile.template"))
	podfile := strings.ReplaceAll(template, "ZORVOK_UPDATABLE_POD_GIT_URL", "file://"+podGitDir)
	if err := os.WriteFile(filepath.Join(projectDir, "Podfile"), []byte(podfile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(projectDir, "Podfile.template")); err != nil {
		t.Fatal(err)
	}

	// First install: locks to the commit that exists right now (1.0.0).
	mustInstall(t, projectDir, nil)
	lockAfterInstall := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if !strings.Contains(lockAfterInstall, "UpdatablePod (1.0.0)") {
		t.Fatalf("expected UpdatablePod (1.0.0) after first install:\n%s", lockAfterInstall)
	}

	// Push a new commit on the same branch bumping the version — a real
	// "someone published a newer version" scenario.
	bumped := strings.ReplaceAll(augmented, "1.0.0", "1.1.0")
	if err := os.WriteFile(podspecPath, []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "v1.1.0")

	// `zorvok install` again must NOT move — it stays locked to what
	// Podfile.lock already recorded, same as real `pod install`.
	mustInstall(t, projectDir, nil)
	lockAfterSecondInstall := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if lockAfterSecondInstall != lockAfterInstall {
		t.Fatalf("plain `zorvok install` must not move a locked branch-tracking dependency forward, but Podfile.lock changed:\n--- before ---\n%s\n--- after ---\n%s", lockAfterInstall, lockAfterSecondInstall)
	}

	// `zorvok update` must move forward to the new commit/version.
	var out, errOut strings.Builder
	if err := installer.Update(&out, &errOut, projectDir, nil); err != nil {
		t.Fatalf("Update failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	lockAfterUpdate := readFile(t, filepath.Join(projectDir, "Podfile.lock"))
	if !strings.Contains(lockAfterUpdate, "UpdatablePod (1.1.0)") {
		t.Fatalf("expected UpdatablePod (1.1.0) after `zorvok update`:\n%s", lockAfterUpdate)
	}
	if lockAfterUpdate == lockAfterInstall {
		t.Fatal("Podfile.lock did not change after `zorvok update` — it should have moved to the new commit")
	}
}
