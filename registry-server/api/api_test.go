package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zorvok-dev/zorvok/registry-server/auth"
	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

// capturingSender records sent verification URLs instead of logging
// them, so tests can drive the full register -> verify -> publish flow
// without a real mail transport.
type capturingSender struct {
	mu   sync.Mutex
	sent map[string]string // email -> verifyURL
}

func newCapturingSender() *capturingSender { return &capturingSender{sent: map[string]string{}} }

func (c *capturingSender) Send(_ context.Context, to, verifyURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent[to] = verifyURL
	return nil
}

func newTestServer() (*httptest.Server, *capturingSender) {
	sender := newCapturingSender()
	s := NewServer(packages.NewMemoryStore(), auth.NewMemoryStore(), sender, "http://testserver")
	ts := httptest.NewServer(s.Routes())
	return ts, sender
}

func doJSON(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	resp.Body.Close()
	return v
}

// registerAndVerify drives the full registration flow a real client
// would, extracting the verify token from the captured "sent" email
// exactly the way a person would click the link.
func registerAndVerify(t *testing.T, ts *httptest.Server, sender *capturingSender, email string) string {
	t.Helper()
	resp := doJSON(t, "POST", ts.URL+"/v1/auth/register", "", registerRequest{Email: email, Name: "Test"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: status %d", resp.StatusCode)
	}
	reg := decode[registerResponse](t, resp)

	sender.mu.Lock()
	verifyURL := sender.sent[email]
	sender.mu.Unlock()
	if verifyURL == "" {
		t.Fatalf("no verification email captured for %s", email)
	}

	verifyResp, err := http.Get(ts.URL + verifyURL[len("http://testserver"):])
	if err != nil {
		t.Fatal(err)
	}
	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("verify: status %d", verifyResp.StatusCode)
	}
	verifyResp.Body.Close()

	return reg.SessionToken
}

func samplePodspec(name, version string) string {
	return fmt.Sprintf(`{"name":%q,"version":%q,"summary":"test","homepage":"https://example.com","license":"MIT","source":{"git":"https://example.com/repo.git","tag":%q}}`, name, version, version)
}

func TestFullPublishFlow(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()

	token := registerAndVerify(t, ts, sender, "dev@example.com")

	resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", token, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
		Source:  "https://github.com/example/Alamofire.git",
	})
	if resp.StatusCode != http.StatusCreated {
		body := decode[map[string]string](t, resp)
		t.Fatalf("publish: status %d, body %v", resp.StatusCode, body)
	}
	v := decode[versionResponse](t, resp)
	if v.Checksum == "" {
		t.Error("expected a server-computed checksum")
	}

	getResp, err := http.Get(ts.URL + "/v1/packages/Alamofire")
	if err != nil {
		t.Fatal(err)
	}
	pkg := decode[packageResponse](t, getResp)
	if len(pkg.Versions) != 1 || pkg.Versions[0] != "1.0.0" {
		t.Fatalf("unexpected versions: %v", pkg.Versions)
	}
	if len(pkg.Owners) != 1 || pkg.Owners[0] != "dev@example.com" {
		t.Fatalf("expected the publisher to own the package, got %v", pkg.Owners)
	}
}

func TestPublishRejectsUnverifiedAccount(t *testing.T) {
	ts, _ := newTestServer()
	defer ts.Close()

	resp := doJSON(t, "POST", ts.URL+"/v1/auth/register", "", registerRequest{Email: "dev@example.com"})
	reg := decode[registerResponse](t, resp)

	publishResp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", reg.SessionToken, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
	})
	if publishResp.StatusCode != http.StatusForbidden {
		t.Fatalf("got status %d, want 403 (unverified account must not be able to publish)", publishResp.StatusCode)
	}
}

func TestPublishRejectsMissingAuth(t *testing.T) {
	ts, _ := newTestServer()
	defer ts.Close()

	resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", "", publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", resp.StatusCode)
	}
}

func TestPublishRejectsDuplicateVersion(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()
	token := registerAndVerify(t, ts, sender, "dev@example.com")

	publish := func() int {
		resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", token, publishRequest{
			Version: "1.0.0",
			Podspec: samplePodspec("Alamofire", "1.0.0"),
		})
		return resp.StatusCode
	}
	if status := publish(); status != http.StatusCreated {
		t.Fatalf("first publish: status %d", status)
	}
	if status := publish(); status != http.StatusConflict {
		t.Fatalf("second publish: status %d, want 409 (immutability)", status)
	}
}

func TestPublishRejectsNonOwner(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()
	aliceToken := registerAndVerify(t, ts, sender, "alice@example.com")
	malloryToken := registerAndVerify(t, ts, sender, "mallory@example.com")

	resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", aliceToken, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("alice's publish: status %d", resp.StatusCode)
	}

	// Mallory tries to hijack the existing package with a new version —
	// this is exactly the "dependency confusion / package takeover"
	// scenario ARCHITECTURE.md §5.4 flags; it must be rejected.
	hijack := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", malloryToken, publishRequest{
		Version: "2.0.0",
		Podspec: samplePodspec("Alamofire", "2.0.0"),
	})
	if hijack.StatusCode != http.StatusForbidden {
		t.Fatalf("got status %d, want 403 (non-owner must not be able to publish)", hijack.StatusCode)
	}
}

func TestPublishRejectsPodspecNameMismatch(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()
	token := registerAndVerify(t, ts, sender, "dev@example.com")

	resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", token, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("SomethingElse", "1.0.0"), // mismatched name
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d, want 422 (podspec name must match the package being published)", resp.StatusCode)
	}
}

func TestAddOwnerRequiresExistingOwnership(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()
	aliceToken := registerAndVerify(t, ts, sender, "alice@example.com")
	malloryToken := registerAndVerify(t, ts, sender, "mallory@example.com")
	registerAndVerify(t, ts, sender, "bob@example.com")

	doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", aliceToken, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
	})

	resp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/owners", malloryToken, map[string]string{"email": "mallory@example.com"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("got status %d, want 403 (non-owner can't add owners)", resp.StatusCode)
	}

	okResp := doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/owners", aliceToken, map[string]string{"email": "bob@example.com"})
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("owner adding a co-owner: status %d", okResp.StatusCode)
	}
}

func TestSearchAndNotFound(t *testing.T) {
	ts, sender := newTestServer()
	defer ts.Close()
	token := registerAndVerify(t, ts, sender, "dev@example.com")
	doJSON(t, "POST", ts.URL+"/v1/packages/Alamofire/versions", token, publishRequest{
		Version: "1.0.0",
		Podspec: samplePodspec("Alamofire", "1.0.0"),
	})

	searchResp, err := http.Get(ts.URL + "/v1/search?q=alamo")
	if err != nil {
		t.Fatal(err)
	}
	results := decode[[]packageResponse](t, searchResp)
	if len(results) != 1 || results[0].Name != "Alamofire" {
		t.Fatalf("unexpected search results: %v", results)
	}

	missingResp, err := http.Get(ts.URL + "/v1/packages/DoesNotExist")
	if err != nil {
		t.Fatal(err)
	}
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", missingResp.StatusCode)
	}
}

func TestRateLimiterRejectsExcessRequests(t *testing.T) {
	sender := newCapturingSender()
	s := NewServer(packages.NewMemoryStore(), auth.NewMemoryStore(), sender, "http://testserver")
	s.limiter = newRateLimiter(3, 1<<30) // tiny limit, effectively-infinite window for a deterministic test
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()

	var lastStatus int
	for i := 0; i < 5; i++ {
		resp, err := http.Get(ts.URL + "/v1/search")
		if err != nil {
			t.Fatal(err)
		}
		lastStatus = resp.StatusCode
		resp.Body.Close()
	}
	if lastStatus != http.StatusTooManyRequests {
		t.Fatalf("got final status %d, want 429 after exceeding the limit", lastStatus)
	}
}
