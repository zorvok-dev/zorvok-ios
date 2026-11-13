package safehttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSecureURLAllowsOnlyHTTPSOrThisMachine(t *testing.T) {
	for _, u := range []string{
		"https://registry.example.com",
		"HTTPS://registry.example.com/",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://127.4.5.6",
		"http://[::1]:8080",
		"http://LOCALHOST",
	} {
		parsed, _ := url.Parse(u)
		if !SecureURL(parsed) {
			t.Errorf("SecureURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"http://registry.example.com",
		"http://localhost.example.com",
		"http://127.0.0.1.example.com",
		"http://10.0.0.5:8080",
		"ftp://registry.example.com",
		"registry.example.com",
		"https://",
		"",
	} {
		parsed, _ := url.Parse(u)
		if SecureURL(parsed) {
			t.Errorf("SecureURL(%q) = true, want false", u)
		}
	}
}

// TestNewClientWontFollowARedirectToPlainHTTP proves a token can't be
// redirected onto plain http: Go would resend Authorization to the same
// host, so the redirect itself is refused.
func TestNewClientWontFollowARedirectToPlainHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://registry.example.com/steal", http.StatusFound)
	}))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer secret")
	_, err := NewClient(5 * time.Second).Do(req)
	if err == nil || !strings.Contains(err.Error(), "refusing to follow a redirect") {
		t.Fatalf("err = %v, want the redirect refused", err)
	}
}

func TestNewClientFollowsARedirectThatStaysSecure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	resp, err := NewClient(5 * time.Second).Get(srv.URL + "/old")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || resp.Request.URL.Path != "/new" {
		t.Fatalf("ended at %s with %d, want /new with 204", resp.Request.URL.Path, resp.StatusCode)
	}
}

func TestPrintableDropsTerminalAndBidiControls(t *testing.T) {
	in := "bad \x1b]8;;https://evil.example\x07link\x1b]8;;\x07\r‮name⁦\n\tnext line\x7f\u009b"
	want := "bad ]8;;https://evil.examplelink]8;;name\n\tnext line"
	if got := Printable(in); got != want {
		t.Fatalf("Printable = %q, want %q", got, want)
	}
}
