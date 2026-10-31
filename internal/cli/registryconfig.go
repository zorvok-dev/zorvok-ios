package cli

import (
	"os"

	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

// defaultRegistryURL is used when no `registry_url` is configured.
// There is no publicly hosted Zorvok registry yet (ROADMAP.md Milestone
// 3/6) — this points at the local dev server `cmd/zorvok-registry` runs
// by default, not a real production endpoint, matching the engineering
// rule against hardcoding production URLs.
const defaultRegistryURL = "http://localhost:8080"

// registryURL resolves the effective registry URL the same way
// `zorvok config` resolves any other setting: project (.zorvok.yml) over
// global (~/.config/zorvok/config.yml) over the local-dev default.
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
		return v, true, nil
	}
	return "", false, nil
}
