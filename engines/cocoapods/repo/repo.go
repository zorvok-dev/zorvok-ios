// Package repo wraps `pod repo list/add/add-cdn/remove/update` for
// `zorvok registry`. This is how a named spec source — git- or
// CDN-backed — gets registered for a Podfile `source` line, or for
// Zorvok's own federation, to point at.
package repo

import (
	"fmt"
	"io"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
)

// List runs `pod repo list`.
func List(out, errOut io.Writer, dir string, extraArgs []string) error {
	if _, err := adapter.Find(); err != nil {
		return err
	}
	return adapter.Run(out, errOut, dir, append([]string{"repo", "list"}, extraArgs...)...)
}

// Add runs `pod repo add <name> <url> [branch]` — registers a
// git-backed spec repo.
func Add(out, errOut io.Writer, dir, name, url string, extraArgs []string) error {
	if name == "" || url == "" {
		return fmt.Errorf("usage: zorvok registry add <name> <url> [branch]")
	}
	if _, err := adapter.Find(); err != nil {
		return err
	}
	args := append([]string{"repo", "add", name, url}, extraArgs...)
	return adapter.Run(out, errOut, dir, args...)
}

// AddCDN runs `pod repo add-cdn <name> <url>` — registers a CDN-backed
// spec repo (the protocol Zorvok's own registry speaks, ADR 0006). This
// is the correct way to register a Zorvok registry as a named source
// for use in a Podfile `source` line, as opposed to Add (git-backed).
func AddCDN(out, errOut io.Writer, dir, name, url string, extraArgs []string) error {
	if name == "" || url == "" {
		return fmt.Errorf("usage: zorvok registry add-cdn <name> <url>")
	}
	if _, err := adapter.Find(); err != nil {
		return err
	}
	args := append([]string{"repo", "add-cdn", name, url}, extraArgs...)
	return adapter.Run(out, errOut, dir, args...)
}

// Remove runs `pod repo remove <name>`.
func Remove(out, errOut io.Writer, dir, name string, extraArgs []string) error {
	if name == "" {
		return fmt.Errorf("usage: zorvok registry remove <name>")
	}
	if _, err := adapter.Find(); err != nil {
		return err
	}
	args := append([]string{"repo", "remove", name}, extraArgs...)
	return adapter.Run(out, errOut, dir, args...)
}

// Update runs `pod repo update [name]` — name is optional; omitted
// updates every registered spec repo.
func Update(out, errOut io.Writer, dir, name string, extraArgs []string) error {
	if _, err := adapter.Find(); err != nil {
		return err
	}
	args := []string{"repo", "update"}
	if name != "" {
		args = append(args, name)
	}
	args = append(args, extraArgs...)
	return adapter.Run(out, errOut, dir, args...)
}
