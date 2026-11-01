package installer

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fixturesDir = "../../../tests/fixtures"

func requirePod(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found")
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping: no local `git` install found")
	}
}

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

func copyRel(t *testing.T, root, rel string) {
	t.Helper()
	copyTree(t, filepath.Join(fixturesDir, rel), filepath.Join(root, rel))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=zorvok-test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=zorvok-test", "GIT_COMMITTER_EMAIL=test@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestInstallNativeMatchesDelegatedInstall is the Milestone 6
// differential test ADR 0002 requires before the native path could ever
// become default: with no extra sources, InstallNative must produce the
// same Podfile.lock as the delegated Install path, because it's driving
// the exact same Pod::Installer class the `pod` CLI itself uses — just
// supplied an in-memory Podfile instead of letting CocoaPods re-read
// the file.
func TestInstallNativeMatchesDelegatedInstall(t *testing.T) {
	requirePod(t)

	delegatedRoot := t.TempDir()
	copyRel(t, delegatedRoot, "local-pod")
	copyRel(t, delegatedRoot, "simple-project")
	delegatedProject := filepath.Join(delegatedRoot, "simple-project")

	var out, errOut strings.Builder
	if err := Install(&out, &errOut, delegatedProject, nil); err != nil {
		t.Fatalf("delegated Install: %v\n%s", err, errOut.String())
	}
	delegatedLock, err := os.ReadFile(filepath.Join(delegatedProject, "Podfile.lock"))
	if err != nil {
		t.Fatal(err)
	}

	nativeRoot := t.TempDir()
	copyRel(t, nativeRoot, "local-pod")
	copyRel(t, nativeRoot, "simple-project")
	nativeProject := filepath.Join(nativeRoot, "simple-project")

	out.Reset()
	errOut.Reset()
	if err := InstallNative(&out, &errOut, nativeProject, nil); err != nil {
		t.Fatalf("InstallNative: %v\n%s", err, errOut.String())
	}
	nativeLock, err := os.ReadFile(filepath.Join(nativeProject, "Podfile.lock"))
	if err != nil {
		t.Fatal(err)
	}

	if string(delegatedLock) != string(nativeLock) {
		t.Fatalf("native and delegated installs produced different Podfile.lock:\n--- delegated ---\n%s\n--- native ---\n%s", delegatedLock, nativeLock)
	}
}

// TestInstallNativeFederatesUndeclaredDependency is the Milestone 6
// payoff test: resolves a pod that is NOT declared with any :path/:git
// in the Podfile, and whose source is NOT named by any `source` line in
// the Podfile either — it's injected entirely in memory by Zorvok's
// shim (ADR 0004/0005) — while proving the Podfile on disk is never
// written to. CP_HOME_DIR isolates this from the real machine's
// ~/.cocoapods (no network access, no shared state with other tests or
// the developer's own CocoaPods config).
func TestInstallNativeFederatesUndeclaredDependency(t *testing.T) {
	requirePod(t)
	requireGit(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()

	podGitDir := filepath.Join(root, "federated-pod-git")
	copyRel(t, root, "federated-pod")
	if err := os.Rename(filepath.Join(root, "federated-pod"), podGitDir); err != nil {
		t.Fatal(err)
	}
	runGit(t, podGitDir, "init", "-q", "-b", "main")
	runGit(t, podGitDir, "add", "-A")
	runGit(t, podGitDir, "commit", "-q", "-m", "init")
	runGit(t, podGitDir, "tag", "1.0.0")

	specsRepoDir := filepath.Join(root, "specs-repo")
	copyRel(t, root, "federated-specs-template")
	if err := os.Rename(filepath.Join(root, "federated-specs-template"), specsRepoDir); err != nil {
		t.Fatal(err)
	}
	podspecPath := filepath.Join(specsRepoDir, "Specs", "FederatedPod", "1.0.0", "FederatedPod.podspec.json")
	content, err := os.ReadFile(podspecPath)
	if err != nil {
		t.Fatal(err)
	}
	augmented := strings.ReplaceAll(string(content), "ZORVOK_FEDERATED_POD_GIT_URL", "file://"+podGitDir)
	if err := os.WriteFile(podspecPath, []byte(augmented), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, specsRepoDir, "init", "-q", "-b", "main")
	runGit(t, specsRepoDir, "add", "-A")
	runGit(t, specsRepoDir, "commit", "-q", "-m", "init")

	copyRel(t, root, "federated-project")
	projectDir := filepath.Join(root, "federated-project")
	podfilePath := filepath.Join(projectDir, "Podfile")
	before, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	err = InstallNative(&out, &errOut, projectDir, []string{"file://" + specsRepoDir})
	if err != nil {
		t.Fatalf("InstallNative: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	after, err := os.ReadFile(podfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("the Podfile on disk was modified — Zorvok must never write to it, even to federate an undeclared dependency")
	}

	lock, err := os.ReadFile(filepath.Join(projectDir, "Podfile.lock"))
	if err != nil {
		t.Fatalf("Podfile.lock was not created: %v", err)
	}
	if !strings.Contains(string(lock), "FederatedPod (1.0.0)") {
		t.Fatalf("Podfile.lock missing FederatedPod:\n%s", lock)
	}
}

// TestInstallNativeEvaluatesPodfileExactlyOnce proves a Podfile's
// top-level Ruby runs once per InstallNative call, the same as a real
// `pod install` would. The shim used to parse the Podfile once just to
// inspect its declared sources, then build an "augmented" Podfile by
// prepending synthetic `source` lines to the same file contents and
// parsing that too — silently running any side-effecting top-level
// code in the file (a `puts`, a shell-out, a hook registered as a
// require's side effect) a second time.
func TestInstallNativeEvaluatesPodfileExactlyOnce(t *testing.T) {
	requirePod(t)
	t.Setenv("CP_HOME_DIR", t.TempDir())

	root := t.TempDir()
	copyRel(t, root, "local-pod")
	copyRel(t, root, "eval-once-project")
	projectDir := filepath.Join(root, "eval-once-project")

	marker := filepath.Join(root, "eval-marker")
	t.Setenv("ZORVOK_EVAL_MARKER", marker)

	var out, errOut strings.Builder
	if err := InstallNative(&out, &errOut, projectDir, nil); err != nil {
		t.Fatalf("InstallNative: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file was never written: %v", err)
	}
	lines := strings.Count(string(data), "\n")
	if lines != 1 {
		t.Fatalf("Podfile's top-level code ran %d time(s), want exactly 1 (marker contents: %q)", lines, data)
	}
}
