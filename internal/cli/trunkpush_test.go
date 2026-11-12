package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zorvok-dev/zorvok-ios/internal/githubtest"
)

const myPodspec = `Pod::Spec.new do |s|
  s.name         = 'MyPod'
  s.version      = '1.0.0'
  s.summary      = 'A pod for testing trunk push.'
  s.homepage     = 'https://github.com/alice/MyPod'
  s.license      = { :type => 'MIT' }
  s.author       = { 'Alice' => 'alice@example.com' }
  s.source       = { :git => 'https://github.com/alice/MyPod.git', :tag => s.version.to_s }
  s.ios.deployment_target = '13.0'
  s.source_files = 'Sources/**/*.swift'
end
`

func requirePod(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found")
	}
}

// isolateConfig gives the test its own empty Zorvok config and working
// directory, so no real registry_url or project .zorvok.yml applies.
func isolateConfig(t *testing.T) string {
	t.Helper()
	t.Setenv("ZORVOK_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

// withoutGitHubLogin removes every way Token could find credentials.
func withoutGitHubLogin(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PATH", t.TempDir())
}

// publicRegistryPod sets up a pod directory and a fake GitHub that plays
// the registry's publish workflow with verdict.
func publicRegistryPod(t *testing.T, verdict, details string) *githubtest.Fake {
	t.Helper()
	requirePod(t)
	dir := isolateConfig(t)
	t.Setenv("LANG", "en_US.UTF-8")
	if err := os.WriteFile(filepath.Join(dir, "MyPod.podspec"), []byte(myPodspec), 0o644); err != nil {
		t.Fatal(err)
	}

	f := githubtest.New(t, "alice")
	f.ActAsPublishWorkflow(verdict, details)
	t.Setenv("GH_TOKEN", f.Token)
	t.Setenv("ZORVOK_GITHUB_API_URL", f.URL)

	old := publishWaitTimeout
	publishWaitTimeout = 5 * time.Second
	t.Cleanup(func() { publishWaitTimeout = old })
	return f
}

// TestTrunkPushPublishesToThePublicRegistryByDefault is the one-command
// flow: with no registry_url configured, `zorvok trunk push` converts the
// podspec with real `pod ipc spec`, commits it (and OWNERS, for a new
// pod) to the publisher's fork, and reports it published once the
// registry's workflow accepts it.
func TestTrunkPushPublishesToThePublicRegistryByDefault(t *testing.T) {
	f := publicRegistryPod(t, "published", "")

	out, errOut, code := run("trunk", "push", "--skip-lint")
	if code != 0 {
		t.Fatalf("code = %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	for _, want := range []string{
		"Published MyPod 1.0.0 to Zorvok's public registry",
		"pod 'MyPod', :source => 'https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout doesn't contain %q:\n%s", want, out)
		}
	}

	files, ok := f.BranchFiles("zorvok-publish/MyPod-1.0.0")
	if !ok {
		t.Fatal("nothing was committed to the fork")
	}
	if spec := files["Specs/MyPod/1.0.0/MyPod.podspec.json"]; !strings.Contains(spec, `"name": "MyPod"`) || !strings.Contains(spec, `"tag": "1.0.0"`) {
		t.Errorf("committed podspec JSON isn't pod ipc spec's output for MyPod.podspec:\n%s", spec)
	}
	if owners := files["Specs/MyPod/OWNERS"]; owners != "alice\n" {
		t.Errorf("OWNERS = %q, want %q", owners, "alice\n")
	}
}

func TestTrunkPushReportsARejectionWithTheRegistrysReasons(t *testing.T) {
	publicRegistryPod(t, "rejected", "- structure: Specs/MyPod/1.0.0/MyPod.podspec.json is not valid JSON")

	_, errOut, code := run("trunk", "push", "--skip-lint")
	if code != 1 {
		t.Fatalf("code = %d, want 1 for a rejected publish", code)
	}
	for _, want := range []string{"didn't accept MyPod 1.0.0", "is not valid JSON", "https://github.com/zorvok-dev/zorvok-specs/pull/1"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr doesn't contain %q:\n%s", want, errOut)
		}
	}
}

func TestTrunkPushReportsAOneTimeReview(t *testing.T) {
	publicRegistryPod(t, "needs-review", "MyPod already exists on CocoaPods trunk.")

	out, errOut, code := run("trunk", "push", "--skip-lint")
	if code != 0 {
		t.Fatalf("code = %d, want 0 (submitted, awaiting review)\nstderr: %s", code, errOut)
	}
	for _, want := range []string{"needs a one-time review", "already exists on CocoaPods trunk"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout doesn't contain %q:\n%s", want, out)
		}
	}
}

func TestTrunkPushWithoutGitHubLoginSaysHowToSignIn(t *testing.T) {
	isolateConfig(t)
	withoutGitHubLogin(t)

	_, errOut, code := run("trunk", "push")
	if code != 1 || !strings.Contains(errOut, "gh auth login") {
		t.Fatalf("code = %d, stderr = %q; want a hint to run `gh auth login`", code, errOut)
	}
}

// TestTrunkPushToThePublicRegistryURLGoesThroughGitHub proves a
// registry_url set to the public registry (as the README suggests for
// installing from it) publishes there too, instead of being treated as a
// registry server needing `zorvok login`.
func TestTrunkPushToThePublicRegistryURLGoesThroughGitHub(t *testing.T) {
	isolateConfig(t)
	if _, errOut, code := run("config", "set", "registry_url", "https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/", "--global"); code != 0 {
		t.Fatalf("config set failed: %s", errOut)
	}
	withoutGitHubLogin(t)

	_, errOut, code := run("trunk", "push")
	if code != 1 || !strings.Contains(errOut, "gh auth login") {
		t.Fatalf("code = %d, stderr = %q; want the public registry's GitHub sign-in hint", code, errOut)
	}
}

// TestTrunkPushToAConfiguredRegistryServerStillUsesZorvokLogin proves a
// registry_url naming a registry server keeps publishing to that server.
func TestTrunkPushToAConfiguredRegistryServerStillUsesZorvokLogin(t *testing.T) {
	isolateConfig(t)
	if _, errOut, code := run("config", "set", "registry_url", "http://127.0.0.1:1", "--global"); code != 0 {
		t.Fatalf("config set failed: %s", errOut)
	}

	_, errOut, code := run("trunk", "push")
	if code != 1 || !strings.Contains(errOut, "not logged in to http://127.0.0.1:1") {
		t.Fatalf("code = %d, stderr = %q; want the registry server's not-logged-in error", code, errOut)
	}
}
