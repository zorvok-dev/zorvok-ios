package cdn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

func newTestStore(t *testing.T) packages.Store {
	t.Helper()
	store := packages.NewMemoryStore()
	ctx := context.Background()

	if _, err := store.GetOrCreate(ctx, "Alamofire", "dev@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishVersion(ctx, packages.Version{
		PackageName: "Alamofire",
		Version:     "1.0.0",
		Podspec:     `{"name":"Alamofire","version":"1.0.0","source":{"git":"https://example.com/Alamofire.git","tag":"1.0.0"}}`,
		PublishedBy: "dev@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishVersion(ctx, packages.Version{
		PackageName: "Alamofire",
		Version:     "2.0.0",
		Podspec:     `{"name":"Alamofire","version":"2.0.0","source":{"git":"https://example.com/Alamofire.git","tag":"2.0.0"}}`,
		PublishedBy: "dev@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestVersionYAMLHasLastAndEmptyPrefixLengths(t *testing.T) {
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/CocoaPods-version.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Real field names `cdn_url?`/`Source::Metadata` parse — verified
	// against the installed cocoapods-core gem's source, not guessed.
	if !strings.Contains(body, "last:") {
		t.Errorf("missing `last:` key, required by Pod::Source::Manager#cdn_url?:\n%s", body)
	}
	if !strings.Contains(body, "prefix_lengths: []") {
		t.Errorf("missing empty prefix_lengths (this registry deliberately disables CDN sharding):\n%s", body)
	}
}

func TestAllPodsListsEveryPackage(t *testing.T) {
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/all_pods.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if strings.TrimSpace(body) != "Alamofire" {
		t.Errorf("got %q, want \"Alamofire\"", body)
	}
}

func TestAllPodsVersionsFormat(t *testing.T) {
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/all_pods_versions.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	line := strings.TrimSpace(string(buf[:n]))

	// Format CDNSource parses: `row.split('/'); pod = row.shift; versions = row`.
	parts := strings.Split(line, "/")
	if parts[0] != "Alamofire" {
		t.Fatalf("unexpected line: %q", line)
	}
	versions := map[string]bool{}
	for _, v := range parts[1:] {
		versions[v] = true
	}
	if !versions["1.0.0"] || !versions["2.0.0"] {
		t.Fatalf("expected both versions listed, got: %q", line)
	}
}

func TestPodspecServedVerbatim(t *testing.T) {
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/Specs/Alamofire/1.0.0/Alamofire.podspec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, `"version":"1.0.0"`) {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestPodspecMismatchedFilenameIs404(t *testing.T) {
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	// A request CDNSource's #specification_path would never construct
	// (filename must be "<name>.podspec.json") — must not accidentally
	// serve anything.
	resp, err := http.Get(ts.URL + "/Specs/Alamofire/1.0.0/SomethingElse.podspec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", resp.StatusCode)
	}
}

func TestPodspecUnknownPodIs404(t *testing.T) {
	// This is the federation fallthrough case in ADR 0003: a pod not on
	// this registry must 404 cleanly so CocoaPods' source aggregation
	// moves on to the next declared source, not error out.
	ts := httptest.NewServer(Handler(newTestStore(t)))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/Specs/DoesNotExist/1.0.0/DoesNotExist.podspec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", resp.StatusCode)
	}
}
