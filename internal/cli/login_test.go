package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeRegistrySrv starts a minimal HTTP server that answers
// /v1/auth/register, capturing the request body it received so a test
// can assert on exactly what runLogin sent.
func fakeRegistrySrv(t *testing.T, gotName *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		*gotName = req.Name
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"session_token": "tok", "email": req.Email, "verified": false})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestLoginJoinsMultiWordDisplayName proves a multi-word display name
// ("Jane Doe") reaches the registry whole. Before this was fixed,
// runLogin took only args[1] as the name — a second word ("Doe") was
// silently dropped on the floor, with no error telling the caller
// their name was truncated.
func TestLoginJoinsMultiWordDisplayName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	var gotName string
	srv := fakeRegistrySrv(t, &gotName)

	run("config", "set", "registry_url", srv.URL, "--global")
	_, errOut, code := run("login", "dev@example.com", "Jane", "Doe")
	if code != 0 {
		t.Fatalf("login failed: code=%d stderr=%q", code, errOut)
	}
	if gotName != "Jane Doe" {
		t.Fatalf("registry received name %q, want %q", gotName, "Jane Doe")
	}
}

// TestLoginWithSingleWordNameStillWorks guards against the join
// introducing a regression for the common single-word case.
func TestLoginWithSingleWordNameStillWorks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	var gotName string
	srv := fakeRegistrySrv(t, &gotName)

	run("config", "set", "registry_url", srv.URL, "--global")
	_, errOut, code := run("login", "dev@example.com", "Jane")
	if code != 0 {
		t.Fatalf("login failed: code=%d stderr=%q", code, errOut)
	}
	if gotName != "Jane" {
		t.Fatalf("registry received name %q, want %q", gotName, "Jane")
	}
}
