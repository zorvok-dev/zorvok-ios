package publisher

import (
	"context"
	"io"
	"io/fs"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zorvok-dev/zorvok/internal/registry"
	regapi "github.com/zorvok-dev/zorvok/registry-server/api"
	regauth "github.com/zorvok-dev/zorvok/registry-server/auth"
	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

const fixturesDir = "../../tests/fixtures"

func requirePod(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pod"); err != nil {
		t.Skip("skipping: no local `pod` install found")
	}
}

type capturingSender struct {
	mu   sync.Mutex
	sent map[string]string
}

func (c *capturingSender) Send(_ context.Context, to, verifyURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent[to] = verifyURL
	return nil
}

func newTestRegistry(t *testing.T) (*registry.Client, string) {
	t.Helper()
	sender := &capturingSender{sent: map[string]string{}}
	server := regapi.NewServer(packages.NewMemoryStore(), regauth.NewMemoryStore(), sender, "http://testserver")
	ts := httptest.NewServer(server.Routes())
	t.Cleanup(ts.Close)

	client := registry.NewClient(ts.URL)
	ctx := context.Background()
	token, err := client.Register(ctx, "dev@example.com", "Dev")
	if err != nil {
		t.Fatal(err)
	}
	sender.mu.Lock()
	verifyURL := sender.sent["dev@example.com"]
	sender.mu.Unlock()
	resp, err := client.HTTP.Get(client.BaseURL + verifyURL[len("http://testserver"):])
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	return client, token
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
		t.Fatalf("copy fixture: %v", err)
	}
}

// TestPushPublishesToARealRunningRegistry is the Milestone 4 acceptance
// test (ROADMAP.md: "Publish a real test podspec end-to-end"): lints a
// real podspec with real `pod lib lint`, converts it with real `pod ipc
// spec`, and publishes it to a real (in-memory, httptest) registry
// server — the same server binary cmd/zorvok-registry runs, not a mock.
func TestPushPublishesToARealRunningRegistry(t *testing.T) {
	requirePod(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join(fixturesDir, "publishable-pod"), dir)

	client, token := newTestRegistry(t)

	var out, errOut strings.Builder
	published, err := Push(context.Background(), &out, &errOut, dir, client, token, Options{})
	if err != nil {
		t.Fatalf("Push failed: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if published.Package != "PublishablePod" || published.Version != "1.0.0" {
		t.Fatalf("unexpected published record: %+v", published)
	}
	if published.Checksum == "" {
		t.Error("expected a non-empty checksum")
	}

	pkg, err := client.GetPackage(context.Background(), "PublishablePod")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if len(pkg.Versions) != 1 || pkg.Versions[0] != "1.0.0" {
		t.Fatalf("unexpected versions: %v", pkg.Versions)
	}
}

// TestPushRejectsLocalPathSource proves the client-side check
// (validateSourceIsFetchable) runs before anything reaches the network —
// LocalPod.podspec's :path source is fine for `pod install` but useless
// for a registry publish.
func TestPushRejectsLocalPathSource(t *testing.T) {
	requirePod(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join(fixturesDir, "local-pod"), dir)

	client, token := newTestRegistry(t)

	var out, errOut strings.Builder
	_, err := Push(context.Background(), &out, &errOut, dir, client, token, Options{SkipLint: true})
	if err == nil {
		t.Fatal("expected Push to reject a podspec whose source is a local :path")
	}
	if !strings.Contains(err.Error(), ":path") {
		t.Fatalf("expected the error to explain the :path problem, got: %v", err)
	}
}

// TestPushFailsLintBeforePublishing proves a podspec that fails local
// lint never reaches the registry at all.
func TestPushFailsLintBeforePublishing(t *testing.T) {
	requirePod(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Broken.podspec"), []byte(`Pod::Spec.new do |s|
  s.name = "Broken"
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	client, token := newTestRegistry(t)

	var out, errOut strings.Builder
	_, err := Push(context.Background(), &out, &errOut, dir, client, token, Options{})
	if err == nil {
		t.Fatal("expected Push to fail lint for a podspec missing required fields")
	}

	if _, getErr := client.GetPackage(context.Background(), "Broken"); getErr == nil {
		t.Fatal("a podspec that failed lint must never reach the registry")
	}
}
