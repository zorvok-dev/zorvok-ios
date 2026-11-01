package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/zorvok-dev/zorvok-ios/internal/registrytest"
)

// newTestRegistry starts a real zorvok-registry subprocess (zorvok-dev/
// zorvok-registry, zorvok ADR 0009) and returns a Client pointed at it —
// these tests exercise the actual HTTP contract against the real
// server binary, not a mock of it.
func newTestRegistry(t *testing.T) (*Client, *registrytest.Server) {
	t.Helper()
	srv := registrytest.Start(t)
	return NewClient(srv.BaseURL), srv
}

func registerAndVerify(t *testing.T, client *Client, srv *registrytest.Server, email string) string {
	t.Helper()
	ctx := context.Background()

	token, err := client.Register(ctx, email, "Test")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	verifyURL := srv.WaitForVerifyLink(t, email)
	resp, err := client.HTTP.Get(verifyURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	return token
}

func TestRegisterAndPublish(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

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
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

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

func TestMe(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

	account, err := client.Me(ctx, token)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if account.Email != "dev@example.com" || !account.Verified {
		t.Fatalf("unexpected account: %+v", account)
	}
}

func TestMeWithoutSessionIsAPIError(t *testing.T) {
	client, _ := newTestRegistry(t)
	_, err := client.Me(context.Background(), "not-a-real-token")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("got %v, want *APIError with status 401", err)
	}
}

func TestAddAndRemoveOwner(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	aliceToken := registerAndVerify(t, client, srv, "alice@example.com")
	registerAndVerify(t, client, srv, "bob@example.com")

	if _, err := client.PublishVersion(ctx, aliceToken, "Alamofire", "1.0.0", `{"name":"Alamofire","version":"1.0.0"}`, ""); err != nil {
		t.Fatal(err)
	}

	if err := client.AddOwner(ctx, aliceToken, "Alamofire", "bob@example.com"); err != nil {
		t.Fatalf("AddOwner: %v", err)
	}
	pkg, err := client.GetPackage(ctx, "Alamofire")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Owners) != 2 {
		t.Fatalf("expected 2 owners after AddOwner, got %v", pkg.Owners)
	}

	if err := client.RemoveOwner(ctx, aliceToken, "Alamofire", "bob@example.com"); err != nil {
		t.Fatalf("RemoveOwner: %v", err)
	}
	pkg, err = client.GetPackage(ctx, "Alamofire")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Owners) != 1 || pkg.Owners[0] != "alice@example.com" {
		t.Fatalf("expected only alice to remain an owner, got %v", pkg.Owners)
	}
}

func TestRemoveLastOwnerIsAPIError(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	aliceToken := registerAndVerify(t, client, srv, "alice@example.com")

	if _, err := client.PublishVersion(ctx, aliceToken, "Alamofire", "1.0.0", `{"name":"Alamofire","version":"1.0.0"}`, ""); err != nil {
		t.Fatal(err)
	}

	err := client.RemoveOwner(ctx, aliceToken, "Alamofire", "alice@example.com")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
		t.Fatalf("got %v, want *APIError with status 409 (cannot remove the last owner)", err)
	}
}

func TestDeprecate(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

	if _, err := client.PublishVersion(ctx, token, "Alamofire", "1.0.0", `{"name":"Alamofire","version":"1.0.0"}`, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.Deprecate(ctx, token, "Alamofire", "use Alamofire2 instead"); err != nil {
		t.Fatalf("Deprecate: %v", err)
	}

	pkg, err := client.GetPackage(ctx, "Alamofire")
	if err != nil {
		t.Fatal(err)
	}
	if !pkg.Deprecated || pkg.DeprecationMessage != "use Alamofire2 instead" {
		t.Fatalf("unexpected package after deprecation: %+v", pkg)
	}
}

// TestPackageNameWithURLMetacharactersReachesTheRightEndpoint proves a
// package name is percent-encoded into the request path, not spliced
// in raw. Before this was fixed, a name containing `?` truncated the
// actual request path at the `?` (everything after it, including the
// intended `/versions` suffix, became query-string garbage instead),
// silently misrouting the request. A name like this is never valid
// (packages.ValidateName rejects it), so the proof is in *how* it
// fails: a clean 400 from the versions-publish route (meaning the
// full path, name included, was correctly received and validated) —
// not some other status shape that would indicate the request landed
// on the wrong route entirely.
func TestPackageNameWithURLMetacharactersReachesTheRightEndpoint(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

	_, err := client.PublishVersion(ctx, token, "Foo?x=1", "1.0.0", `{"name":"Foo?x=1","version":"1.0.0"}`, "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("got %v, want *APIError with status 400 (invalid name, correctly routed) - a misrouted request would fail differently", err)
	}
}

func TestSearch(t *testing.T) {
	client, srv := newTestRegistry(t)
	ctx := context.Background()
	token := registerAndVerify(t, client, srv, "dev@example.com")

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
