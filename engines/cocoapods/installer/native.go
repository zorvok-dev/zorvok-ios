package installer

import (
	_ "embed"
	"io"

	"github.com/zorvok-dev/zorvok-ios/engines/cocoapods/adapter"
)

//go:embed zorvok_install.rb
var nativeInstallScript string

// InstallNative runs Zorvok's own installer shim (ADR 0002 Phase 3,
// Milestone 6) instead of delegating to the `pod install` binary. With
// no extraSources it is behaviorally equivalent to Install — verified
// by a differential test comparing Podfile.lock output byte-for-byte —
// so this is additive, not a replacement: Install (delegated) remains
// the default, InstallNative is opt-in (`zorvok install --native`) per
// ADR 0002's "ships behind a flag... before becoming default."
//
// extraSources are additional CocoaPods spec source URLs (git or CDN)
// to search, beyond whatever the real Podfile already declares — the
// mechanism that makes registry federation possible without ever
// writing to the Podfile on disk (see zorvok_install.rb's own comment,
// and ADR 0005).
func InstallNative(out, errOut io.Writer, dir string, extraSources []string) error {
	if err := requirePodfile(dir); err != nil {
		return err
	}
	args := append([]string{dir}, extraSources...)
	return adapter.RunRubyScript(out, errOut, dir, nativeInstallScript, args...)
}
