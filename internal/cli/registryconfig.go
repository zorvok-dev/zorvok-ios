package cli

import (
	"os"

	"github.com/zorvok-dev/zorvok/internal/config"
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
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		return "", err
	}
	g, err := config.Load(globalPath)
	if err != nil {
		return "", err
	}
	p, err := config.Load(config.ProjectPath(cwd))
	if err != nil {
		return "", err
	}
	merged := config.Merged(g, p)
	if v, ok := merged.Get("registry_url"); ok && v != "" {
		return v, nil
	}
	return defaultRegistryURL, nil
}
