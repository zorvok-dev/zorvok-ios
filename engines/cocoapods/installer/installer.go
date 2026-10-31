// Package installer implements `zorvok install`/`zorvok update` for
// Milestone 2: Phase 1 of ADR 0002, meaning it delegates resolution and
// Xcode integration entirely to the real `pod install`/`pod update`
// binary rather than reimplementing them. Zorvok's value here is the
// preflight checks and error messages around that delegation, not the
// installation logic itself.
package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zorvok-dev/zorvok/engines/cocoapods/adapter"
)

// ErrNoPodfile is returned when dir has no Podfile. Zorvok manages
// existing CocoaPods projects; it does not scaffold a new Podfile.
var ErrNoPodfile = fmt.Errorf("no Podfile found")

func requirePodfile(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "Podfile")); err != nil {
		return fmt.Errorf("%w in %s — zorvok manages existing CocoaPods projects; run `pod init` first, or add a Podfile manually", ErrNoPodfile, dir)
	}
	return nil
}

// Install runs `pod install` in dir, forwarding extraArgs verbatim
// (e.g. --repo-update, --verbose) after verifying the project looks like
// a CocoaPods project and `pod` is available.
func Install(out, errOut io.Writer, dir string, extraArgs []string) error {
	if err := requirePodfile(dir); err != nil {
		return err
	}
	if _, err := adapter.Find(); err != nil {
		return err
	}
	return adapter.Run(out, errOut, dir, append([]string{"install"}, extraArgs...)...)
}

// Update runs `pod update` in dir, forwarding extraArgs verbatim — this
// includes specific pod names (`zorvok update Alamofire`), which `pod
// update` already supports natively.
func Update(out, errOut io.Writer, dir string, extraArgs []string) error {
	if err := requirePodfile(dir); err != nil {
		return err
	}
	if _, err := adapter.Find(); err != nil {
		return err
	}
	return adapter.Run(out, errOut, dir, append([]string{"update"}, extraArgs...)...)
}
