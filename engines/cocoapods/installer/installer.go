// Package installer implements `zorvok install`/`update` by delegating
// resolution and Xcode integration to the real `pod install`/`update`
// binary. It owns the preflight checks and error messages around that
// delegation, not the install logic itself.
package installer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
)

// ErrNoPodfile is returned when dir has no Podfile. Zorvok manages
// existing CocoaPods projects; it does not scaffold a new Podfile.
var ErrNoPodfile = errors.New("no Podfile found")

func requirePodfile(dir string) error {
	info, err := os.Stat(filepath.Join(dir, "Podfile"))
	if err != nil || info.IsDir() {
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
