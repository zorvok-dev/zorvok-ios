package registry

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"

	regapi "github.com/zorvok-dev/zorvok/registry-server/api"
	regauth "github.com/zorvok-dev/zorvok/registry-server/auth"
	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

// capturingSender records verification links instead of sending real
// email, so tests can drive register -> verify without a mail
// transport — mirroring registry-server/api's own test helper, since
// these tests exercise the real server, not a mock of it.
type capturingSender struct {
	mu   sync.Mutex
	sent map[string]string
}

func newCapturingSender() *capturingSender { return &capturingSender{sent: map[string]string{}} }

func (c *capturingSender) Send(_ context.Context, to, verifyURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent[to] = verifyURL
	return nil
}

// newTestRegistry starts a real registry-server/api server (in-memory
// store) and returns a Client pointed at it, so these tests exercise
// the actual HTTP contract end-to-end rather than a mock of it.
func newTestRegistry(t *testing.T) (*Client, *capturingSender) {
	t.Helper()
	sender := newCapturingSender()
	server := regapi.NewServer(packages.NewMemoryStore(), regauth.NewMemoryStore(), sender, "http://testserver")
	ts := httptest.NewServer(server.Routes())
	t.Cleanup(ts.Close)
	return NewClient(ts.URL), sender
}

func registerAndVerify(t *testing.T, client *Client, sender *capturingSender, email string) string {
	t.Helper()
	ctx := context.Background()

	token, err := client.Register(ctx, email, "Test")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	sender.mu.Lock()
	verifyURL := sender.sent[email]
	sender.mu.Unlock()
	if verifyURL == "" {
		t.Fatal("no verification email captured")
	}
	// The link is served by the same registry, just hit it directly.
	resp, err := client.HTTP.Get(client.BaseURL + verifyURL[len("http://testserver"):])
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	return token
}

func TestRegisterAndPublish(t *testing.T) {
	client, sender := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, sender, "dev@example.com")

	podspec := `{"name":"Alamofire","version":"1.0.0"}`
	published, err := client.PublishVersion(ctx, token, "Alamofire", "1.0.0", podspec, "https://github.com/example/Alamofire.git")
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	if published.Checksum == "" {
		t.Error("expected a non-empty checksum")
	}

	pkg, err := client.GetPackage(ctx, "Alamofire")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if len(pkg.Versions) != 1 || pkg.Versions[0] != "1.0.0" {
		t.Fatalf("unexpected versions: %v", pkg.Versions)
	}
}

func TestPublishDuplicateVersionIsAPIError(t *testing.T) {
	client, sender := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, sender, "dev@example.com")

	podspec := `{"name":"Alamofire","version":"1.0.0"}`
	if _, err := client.PublishVersion(ctx, token, "Alamofire", "1.0.0", podspec, ""); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	_, err := client.PublishVersion(ctx, token, "Alamofire", "1.0.0", podspec, "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
		t.Fatalf("got %v, want *APIError with status 409", err)
	}
}

func TestPublishBeforeVerifyIsAPIError(t *testing.T) {
	client, _ := newTestRegistry(t)
	ctx := context.Background()

	token, err := client.Register(ctx, "dev@example.com", "Test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PublishVersion(ctx, token, "Alamofire", "1.0.0", `{"name":"Alamofire","version":"1.0.0"}`, "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 {
		t.Fatalf("got %v, want *APIError with status 403 (unverified account)", err)
	}
}

func TestGetPackageNotFound(t *testing.T) {
	client, _ := newTestRegistry(t)
	_, err := client.GetPackage(context.Background(), "DoesNotExist")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Fatalf("got %v, want *APIError with status 404", err)
	}
}

func TestSearch(t *testing.T) {
	client, sender := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, sender, "dev@example.com")

	for _, name := range []string{"Alamofire", "SDWebImage"} {
		if _, err := client.PublishVersion(ctx, token, name, "1.0.0", `{"name":"`+name+`","version":"1.0.0"}`, ""); err != nil {
			t.Fatal(err)
		}
	}

	results, err := client.Search(ctx, "ala")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Alamofire" {
		t.Fatalf("unexpected results: %+v", results)
	}

	all, err := client.Search(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 results for an empty query, got %d", len(all))
	}
}
