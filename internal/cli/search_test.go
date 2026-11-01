package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakePackageInfoSrv(t *testing.T, name string, owners, versions []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"name":     name,
			"owners":   owners,
			"versions": versions,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSearchRejectsExtraPositionalArgs proves an unquoted multi-word
// query ("zorvok search foo bar") errors instead of silently searching
// for only "foo" and dropping "bar" with no indication anything was
// ignored.
func TestSearchRejectsExtraPositionalArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("search", "foo", "bar")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, `extra argument "bar"`) {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

// TestInfoRejectsExtraPositionalArgs is the same gap for `zorvok info`.
func TestInfoRejectsExtraPositionalArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	_, errOut, code := run("info", "Alamofire", "extra")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, `extra argument "extra"`) {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

// TestLatestVersionPicksBySemverNotArrayOrder proves "latest" is
// computed by numeric version comparison, not by trusting the registry
// response's array order. Before this was fixed, `zorvok search` always
// reported Versions[len-1] as "latest" — here that's "9.0.0", the last
// element but numerically older than "10.0.0".
func TestLatestVersionPicksBySemverNotArrayOrder(t *testing.T) {
	got := latestVersion([]string{"1.0.0", "10.0.0", "9.0.0"})
	if got != "10.0.0" {
		t.Fatalf("latestVersion = %q, want %q", got, "10.0.0")
	}
}

func TestLatestVersionSingleElement(t *testing.T) {
	if got := latestVersion([]string{"2.3.4"}); got != "2.3.4" {
		t.Fatalf("latestVersion = %q, want %q", got, "2.3.4")
	}
}

// TestSearchReportsSemverLatestVersion is the end-to-end version of
// the above, through the actual `zorvok search` command.
func TestSearchReportsSemverLatestVersion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{
			"name":     "Alamofire",
			"owners":   []string{"dev@example.com"},
			"versions": []string{"1.0.0", "10.0.0", "9.0.0"},
		}})
	}))
	t.Cleanup(srv.Close)

	run("config", "set", "registry_url", srv.URL, "--global")
	out, errOut, code := run("search", "ala")
	if code != 0 {
		t.Fatalf("search failed: code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "latest 10.0.0") {
		t.Fatalf("expected latest 10.0.0 in output, got %q", out)
	}
}

// TestInfoOwnersEmptyFallback proves `zorvok info` reports an empty
// Owners list the same way it already reports an empty Versions list
// ("none published") rather than printing a bare, blank line.
func TestInfoOwnersEmptyFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	srv := fakePackageInfoSrv(t, "Alamofire", nil, nil)
	run("config", "set", "registry_url", srv.URL, "--global")

	out, errOut, code := run("info", "Alamofire")
	if code != 0 {
		t.Fatalf("info failed: code=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "Owners:   none") {
		t.Fatalf("expected an explicit empty-owners fallback, got %q", out)
	}
}
