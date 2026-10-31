# ADR 0005: Native installer (ADR 0002 Phase 3) and the federation mechanism it unlocks

## Status
Accepted (Milestone 6, first increment)

## Context
ADR 0004 established that `pod install` cannot be federated across registries without a `source` line in the Podfile on disk, and that editing-then-reverting the Podfile was rejected as violating Zorvok's core invariant. ADR 0002 already named the way out: Phase 3, where Zorvok's own Ruby code drives `cocoapods-core`/`molinillo`/`xcodeproj` directly instead of going through the `pod` CLI binary. This ADR records the concrete mechanism, verified empirically before being built on.

## Decision

### Mechanism
The real `pod install` command (`Pod::Command::Install#run` in the `cocoapods` gem) does, in essence:

```ruby
installer = Installer.new(config.sandbox, config.podfile, config.lockfile)
installer.install!
```

`config.podfile` is normally memoized from `Podfile.from_file(podfile_path)` — reading the literal file on disk. Two facts, both verified directly against the installed `cocoapods`/`cocoapods-core` gems (not assumed from documentation):

1. `Pod::Config#podfile` has a public `attr_writer`.
2. `Pod::Podfile.from_ruby(path, contents = nil)` evaluates `contents` if given, and **only reads `path` from disk if `contents` is nil**.

So: Zorvok's shim (`engines/cocoapods/installer/zorvok_install.rb`) reads the real Podfile's content itself (one read, no write), prepends any federated `source 'url'` lines to that content as a Ruby string, builds a `Pod::Podfile` from the augmented string via `from_ruby(real_path, augmented_contents)`, assigns it to `config.podfile`, and calls `Pod::Installer.new(...).install!` — the exact same call the `pod` CLI itself makes. The file at `real_path` is never opened for writing; CocoaPods never reads it a second time either, since `contents` was supplied.

This is still "delegating to CocoaPods' real implementation" per ADR 0002, not a reimplementation: Molinillo still resolves, `cocoapods-downloader` still downloads, `xcodeproj` still generates the Xcode project. The only change from Phase 1-2 is who constructs the `Podfile` object and who decides which sources it declares.

### Verified before shipping
- `TestInstallNativeMatchesDelegatedInstall`: with zero extra sources, `InstallNative` produces a **byte-identical** `Podfile.lock` to the delegated `Install` path, confirmed against this fixture set — this is the "ships behind a flag... with differential testing" requirement from ADR 0002, satisfied for the baseline case before federation was tested at all.
- `TestInstallNativeFederatesUndeclaredDependency`: a pod declared with no `:path`/`:git` and no `source` line anywhere in the Podfile resolves successfully when its source is passed via `--source`, injected only in memory. The Podfile on disk is hashed before and after and asserted unchanged.
- Both tests run fully isolated from the developer's real CocoaPods environment via `CP_HOME_DIR` (a CocoaPods-native override, not a Zorvok invention) pointed at a temp directory — no network access, no shared state with the machine's real `~/.cocoapods`.
- The same flow was additionally run by hand through the compiled `zorvok` binary (`zorvok install --native --source file://...`) against a real `pod`/CocoaPods install, not just in Go tests.

### CLI surface
`zorvok install --native [--source <url> ...]`. Default behavior (no `--native`) is unchanged — still the delegated Phase 1-2 path. `--source` without `--native` is a hard error, not a silent no-op, since it would otherwise look like it did something.

## Consequences
- Milestone 5's original goal (automatic, zero-Podfile-touch federation) is now technically achievable, via `--native --source`. It remains **opt-in** (both because ADR 0002 requires the flag gate, and because the Zorvok registry doesn't yet speak the CocoaPods CDN/Specs protocol a `--source` URL needs to respond to — see ROADMAP.md "not yet done").
- This increment only proves the mechanism against a hand-built local Specs repo (`tests/fixtures/federated-specs-template`, git-based). Making `registry-server` itself servable as a `--source` target (exposing `all_pods.txt`-shaped index files and per-pod podspec JSON at the paths CocoaPods expects, per ARCHITECTURE.md §2.4's CDN design) is separate, tracked work, not done here.
- The dependency-confusion adversarial review ARCHITECTURE.md §5 calls for is still required before any form of *automatic* (non-explicit-source) federation — e.g. Zorvok auto-adding its own registry as a `--source` by default — ships. This increment only makes explicit, developer-specified `--source` federation possible; it does not itself decide precedence across multiple sources (ADR 0003's precedence rules are not yet implemented against real multi-source conflicts).
- `Pod::Config.instance` is process-global mutable state inside the Ruby shim; each `zorvok install --native` invocation is a fresh `ruby -` process (per `adapter.RunRubyScript`), so this doesn't leak across installs, but it means the shim can't easily be used to install multiple projects within one Ruby process — a non-issue for a CLI tool that already runs one `pod`-equivalent invocation per command anyway.
