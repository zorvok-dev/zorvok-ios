package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupGitPodSource copies tests/fixtures/<fixtureRel> into dir, git-inits
// it, and tags it 1.0.0 — a real, fetchable git repo for a pod's :source.
func setupGitPodSource(t *testing.T, root, fixtureRel, dirName string) string {
	t.Helper()
	dir := filepath.Join(root, dirName)
	copyTree(t, filepath.Join(fixturesDir, fixtureRel), dir)
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	runGit(t, dir, "tag", "1.0.0")
	return dir
}

// setupGitSpecsRepo copies a Specs-repo-template fixture into dir,
// substitutes placeholder in its one podspec.json, and git-inits it.
func setupGitSpecsRepo(t *testing.T, root, templateRel, dirName, podspecRelPath, placeholder, replacement string) string {
	t.Helper()
	dir := filepath.Join(root, dirName)
	copyTree(t, filepath.Join(fixturesDir, templateRel), dir)

	podspecPath := filepath.Join(dir, podspecRelPath)
	content, err := os.ReadFile(podspecPath)
	if err != nil {
		t.Fatal(err)
	}
	augmented := strings.ReplaceAll(string(content), placeholder, replacement)
	if err := os.WriteFile(podspecPath, []byte(augmented), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// TestInstallNativeRejectsSourceURLInjection proves a `--source` value
// (which can come from a project's own configured registry_url, not
// just a flag a developer typed by hand) can't break out of the
// generated Podfile text and run arbitrary Ruby. The shim builds
// `source '<url>'\n` and hands the result to Pod::Podfile.from_ruby,
// which evals it — a URL containing a single quote used to terminate
// the string early and run anything after it.
func TestInstallNativeRejectsSourceURLInjection(t *testing.T) {
	requirePod(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Podfile"), []byte("platform :ios, '13.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "pwned")
	payload := "https://example.com/specs.git'; system(%q{touch " + marker + "}); x='"

	var out, errOut strings.Builder
	// The malicious source can never resolve to anything real, so this
	// is expected to fail — the only thing under test is that it fails
	// WITHOUT running the injected command.
	_ = InstallNative(&out, &errOut, dir, []string{payload})

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("injected Ruby executed — the malicious source URL broke out of the generated Podfile source line")
	}
}

// TestInstallNativeAbortsOnAmbiguousSource is the Milestone 6 security
// fix (ADR 0007): two sources both have a spec for the same pod/version,
// with no explicit `:source =>` pin. Real CocoaPods only warns and
// silently picks one by declaration order (confirmed empirically before
// this fix was written) — that's not safe enough for a tool whose whole
// purpose is introducing a second (federated) source alongside the
// first. The native installer must refuse instead.
func TestInstallNativeAbortsOnAmbiguousSource(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()
	podA := setupGitPodSource(t, root, "ambiguous/pod-a", "pod-a-git")
	podB := setupGitPodSource(t, root, "ambiguous/pod-b", "pod-b-git")
	specsA := setupGitSpecsRepo(t, root, "ambiguous/specs-a-template", "specs-a",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_A_GIT_URL", "file://"+podA)
	specsB := setupGitSpecsRepo(t, root, "ambiguous/specs-b-template", "specs-b",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_B_GIT_URL", "file://"+podB)

	copyRel(t, root, "ambiguous/project")
	projectDir := filepath.Join(root, "ambiguous", "project")
	podfilePath := filepath.Join(projectDir, "Podfile")
	before, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	err = InstallNative(&out, &errOut, projectDir, []string{"file://" + specsA, "file://" + specsB})
	if err == nil {
		t.Fatal("expected InstallNative to refuse an ambiguous (multi-source, unpinned) dependency")
	}
	if !strings.Contains(errOut.String(), "SharedPod") {
		t.Errorf("expected the error to name the ambiguous pod, got stderr: %s", errOut.String())
	}

	if _, err := os.Stat(filepath.Join(projectDir, "Podfile.lock")); err == nil {
		t.Error("Podfile.lock must not be written when installation is refused")
	}
	after, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("the Podfile on disk was modified")
	}
}

// TestInstallNativeAbortsOnTransitiveAmbiguousSource proves the
// ambiguity check also catches a collision introduced transitively —
// a pod declared nowhere in the Podfile, only pulled in by another
// pod's own `dependency`. The check used to look only at the
// Podfile's own root-level dependencies, so this exact shape (the most
// common one in a real dependency graph) went unchecked; it's provably
// not just a narrower case of the root-level test above, since the
// old code passed this scenario silently — see git history/ADR 0007.
func TestInstallNativeAbortsOnTransitiveAmbiguousSource(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()
	podA := setupGitPodSource(t, root, "ambiguous/pod-a", "pod-a-git")
	podB := setupGitPodSource(t, root, "ambiguous/pod-b", "pod-b-git")
	specsA := setupGitSpecsRepo(t, root, "ambiguous/specs-a-template", "specs-a",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_A_GIT_URL", "file://"+podA)
	specsB := setupGitSpecsRepo(t, root, "ambiguous/specs-b-template", "specs-b",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_B_GIT_URL", "file://"+podB)

	wrapper := setupGitPodSource(t, root, "transitive-ambiguous/wrapper-pod", "wrapper-pod-git")
	specsWrapper := setupGitSpecsRepo(t, root, "transitive-ambiguous/specs-wrapper-template", "specs-wrapper",
		"Specs/WrapperPod/1.0.0/WrapperPod.podspec.json", "ZORVOK_TRANSITIVE_WRAPPER_GIT_URL", "file://"+wrapper)

	copyRel(t, root, "transitive-ambiguous/project")
	projectDir := filepath.Join(root, "transitive-ambiguous", "project")
	podfilePath := filepath.Join(projectDir, "Podfile")
	before, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	// SharedPod is named nowhere in these --source args' declaration
	// order relative to the Podfile — only WrapperPod's own podspec
	// pulls it in.
	err = InstallNative(&out, &errOut, projectDir, []string{"file://" + specsWrapper, "file://" + specsA, "file://" + specsB})
	if err == nil {
		t.Fatal("expected InstallNative to refuse a transitively-introduced ambiguous dependency")
	}
	if !strings.Contains(errOut.String(), "SharedPod") {
		t.Errorf("expected the error to name the ambiguous transitive pod, got stderr: %s", errOut.String())
	}

	after, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("the Podfile on disk was modified")
	}
}

// TestInstallNativePinnedSourceBypassesAmbiguityCheck proves the fix
// isn't overly broad: a dependency explicitly pinned to a source via
// `:source =>` is never ambiguous, even when other sources also have a
// same-named pod — it must still install normally.
func TestInstallNativePinnedSourceBypassesAmbiguityCheck(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()
	podA := setupGitPodSource(t, root, "ambiguous/pod-a", "pod-a-git")
	podB := setupGitPodSource(t, root, "ambiguous/pod-b", "pod-b-git")
	specsA := setupGitSpecsRepo(t, root, "ambiguous/specs-a-template", "specs-a",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_A_GIT_URL", "file://"+podA)
	specsB := setupGitSpecsRepo(t, root, "ambiguous/specs-b-template", "specs-b",
		"Specs/SharedPod/1.0.0/SharedPod.podspec.json", "ZORVOK_AMBIGUOUS_POD_B_GIT_URL", "file://"+podB)

	copyRel(t, root, "ambiguous/project-pinned")
	projectDir := filepath.Join(root, "ambiguous", "project-pinned")

	template, err := os.ReadFile(filepath.Join(projectDir, "Podfile.template"))
	if err != nil {
		t.Fatal(err)
	}
	podfile := strings.ReplaceAll(string(template), "ZORVOK_SPECS_A_URL", "file://"+specsA)
	if err := os.WriteFile(filepath.Join(projectDir, "Podfile"), []byte(podfile), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	err = InstallNative(&out, &errOut, projectDir, []string{"file://" + specsA, "file://" + specsB})
	if err != nil {
		t.Fatalf("InstallNative: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	lock, err := os.ReadFile(filepath.Join(projectDir, "Podfile.lock"))
	if err != nil {
		t.Fatalf("Podfile.lock was not created: %v", err)
	}
	if !strings.Contains(string(lock), "SharedPod (1.0.0)") {
		t.Fatalf("Podfile.lock missing SharedPod:\n%s", lock)
	}
}

// TestInstallNativePreservesImplicitDefaultSource is the Milestone 6
// functional-correctness fix (ADR 0007): CocoaPods' own
// Analyzer#sources only falls back to the implicit default source when
// the Podfile declares none of its own — the instant a source is
// injected, that fallback is bypassed unless re-added explicitly. A
// Podfile with a normal, non-federated dependency (resolvable only from
// the implicit default) alongside a federated one must still install
// both. ZORVOK_DEFAULT_SOURCE_URL substitutes a local fixture for the
// real cdn.cocoapods.org so this is testable hermetically.
func TestInstallNativePreservesImplicitDefaultSource(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()

	defaultPodDir := setupGitPodSource(t, root, "default-source-pod", "default-source-pod-git")
	defaultSpecsDir := setupGitSpecsRepo(t, root, "default-source-template", "default-specs",
		"Specs/DefaultSourcePod/1.0.0/DefaultSourcePod.podspec.json",
		"ZORVOK_DEFAULT_SOURCE_POD_GIT_URL", "file://"+defaultPodDir)
	t.Setenv("ZORVOK_DEFAULT_SOURCE_URL", "file://"+defaultSpecsDir)

	federatedPodDir := setupGitPodSource(t, root, "federated-pod", "federated-pod-git")
	federatedSpecsDir := setupGitSpecsRepo(t, root, "federated-specs-template", "federated-specs",
		"Specs/FederatedPod/1.0.0/FederatedPod.podspec.json",
		"ZORVOK_FEDERATED_POD_GIT_URL", "file://"+federatedPodDir)

	copyRel(t, root, "mixed-federation-project")
	projectDir := filepath.Join(root, "mixed-federation-project")

	var out, errOut strings.Builder
	err := InstallNative(&out, &errOut, projectDir, []string{"file://" + federatedSpecsDir})
	if err != nil {
		t.Fatalf("InstallNative: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	lock, err := os.ReadFile(filepath.Join(projectDir, "Podfile.lock"))
	if err != nil {
		t.Fatalf("Podfile.lock was not created: %v", err)
	}
	lockStr := string(lock)
	if !strings.Contains(lockStr, "DefaultSourcePod (1.0.0)") {
		t.Errorf("DefaultSourcePod (resolvable only via the implicit default source) was not installed — the default was silently dropped:\n%s", lockStr)
	}
	if !strings.Contains(lockStr, "FederatedPod (1.0.0)") {
		t.Errorf("FederatedPod missing:\n%s", lockStr)
	}
}
