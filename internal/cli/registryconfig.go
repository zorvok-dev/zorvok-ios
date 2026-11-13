package cli

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

// checkTokenURL refuses to send a token, or an account's sign-up, to u
// unless the connection is encrypted: u must be https, or http to this
// machine (a local dev registry such as defaultRegistryURL, or a test's
// fake), where nobody else is on the path. what names the setting u came
// from, for the error.
func checkTokenURL(what, u string) error {
	parsed, err := url.Parse(u)
	if err == nil && parsed.Host != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "https":
			return nil
		case "http":
			host := strings.ToLower(parsed.Hostname())
			if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
				return nil
			}
		}
	}
	return fmt.Errorf("%s %q must be an https URL, since your login is sent to it (plain http is allowed only to this machine, like %s)", what, u, defaultRegistryURL)
}

// defaultRegistryURL is used when no `registry_url` is configured.
// There is no publicly hosted Zorvok registry yet (ROADMAP.md Milestone
// 3/6) — this points at the local dev server `cmd/zorvok-registry` runs
// by default, not a real production endpoint, matching the engineering
// rule against hardcoding production URLs.
const defaultRegistryURL = "http://localhost:8080"

// registryURL resolves the effective registry URL the same way
// `zorvok config` resolves any other setting: project (.zorvok.yml) over
// global (~/Library/Application Support/zorvok/config.yml on macOS) over
// the local-dev default.
func registryURL() (string, error) {
	url, _, err := registryURLExplicit()
	if err != nil {
		return "", err
	}
	if url == "" {
		return defaultRegistryURL, nil
	}
	return url, nil
}

// registryURLExplicit returns the configured registry_url and whether it
// was actually set by the user (project or global `zorvok config set`),
// as opposed to falling back to defaultRegistryURL. ADR 0008's automatic
// federation deliberately only activates on an explicit configuration —
// never on the bundled local-dev fallback nobody asked for.
func registryURLExplicit() (url string, explicit bool, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		return "", false, err
	}
	g, err := config.Load(globalPath)
	if err != nil {
		return "", false, err
	}
	p, err := config.Load(config.ProjectPath(cwd))
	if err != nil {
		return "", false, err
	}
	merged := config.Merged(g, p)
	if v, ok := merged.Get("registry_url"); ok && v != "" {
		// A trailing slash is cosmetic to whoever typed `registry_url:
		// https://example.com/` — but every call site below builds
		// paths as url+"/v1/...", so left untrimmed it would produce
		// a different (and wrong) URL than the slash-free spelling of
		// the same registry.
		return strings.TrimRight(v, "/"), true, nil
	}
	return "", false, nil
}
