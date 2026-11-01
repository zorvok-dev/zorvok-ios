package cli

import "testing"

func TestRegistryURLExplicitFalseByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	url, explicit, err := registryURLExplicit()
	if err != nil {
		t.Fatal(err)
	}
	if explicit || url != "" {
		t.Fatalf("got (%q, %v), want (\"\", false) when nothing was configured", url, explicit)
	}

	// registryURL() still falls back to the bundled default.
	fallback, err := registryURL()
	if err != nil {
		t.Fatal(err)
	}
	if fallback != defaultRegistryURL {
		t.Fatalf("got %q, want the bundled default %q", fallback, defaultRegistryURL)
	}
}

func TestRegistryURLExplicitTrueAfterConfigSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	run("config", "set", "registry_url", "https://registry.example/cdn", "--global")

	url, explicit, err := registryURLExplicit()
	if err != nil {
		t.Fatal(err)
	}
	if !explicit || url != "https://registry.example/cdn" {
		t.Fatalf("got (%q, %v), want (\"https://registry.example/cdn\", true)", url, explicit)
	}
}

// TestRegistryURLStripsTrailingSlash proves a configured registry_url
// with a trailing slash resolves to the same string as without one.
// Without this, `zorvok login` (which stores its session under
// registryURL()'s exact return value) and a later command reading a
// differently-slashed spelling of the same registry_url would disagree
// on the auth store key and the later command would wrongly report
// "not logged in" to a registry the user is, in fact, logged in to.
func TestRegistryURLStripsTrailingSlash(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	run("config", "set", "registry_url", "https://registry.example/cdn/", "--global")

	url, _, err := registryURLExplicit()
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://registry.example/cdn" {
		t.Fatalf("got %q, want the trailing slash stripped", url)
	}
}
