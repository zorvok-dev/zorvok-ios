# Zorvok Roadmap

Status key: ✅ done · 🚧 in progress · ⬜ not started

## Milestone 0 — Architecture *(this milestone)*
- ✅ Research CocoaPods internals (CLI, CocoaPods-Core, Molinillo, cocoapods-downloader, xcodeproj, Trunk, CDN, Podfile.lock format)
- ✅ Select CLI language ([ADR 0001](docs/adr/0001-cli-language-go.md))
- ✅ Define CLI-to-engine interface ([ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md))
- ✅ Define registry architecture ([ADR 0003](docs/adr/0003-registry-federation-and-storage.md))
- ✅ Architecture document with component diagram, trade-offs, risks ([ARCHITECTURE.md](ARCHITECTURE.md))
- ✅ **Architecture approved**

## Milestone 1 — CLI skeleton ✅
`zorvok --version`, `--help`, `doctor`, `config`, `init`. No CocoaPods interaction yet — `install`/`update`/`search`/`info`/`lib lint`/`login`/`trunk push`/`registry *` all exist as named, documented, explicitly "not implemented yet" commands rather than failing as unknown or silently no-op'ing. Built in Go (module `github.com/zorvok-dev/zorvok`, placeholder path pending a real org), stdlib-only, unit tested.

## Milestone 2 — CocoaPods compatibility 🚧
`zorvok install`, `zorvok update`, `zorvok lib lint` are implemented, delegating to real `pod install`/`pod update`/`pod lib lint` (ADR 0002, Phase 1) via `engines/cocoapods/{adapter,installer,podspec}`. A compatibility test (`engines/cocoapods/compatibility`) runs a real, unmodified `pod install` end-to-end — against actual CocoaPods, not a mock — confirming the Podfile is never touched, `Podfile.lock` gets the expected sections, and `Pods/`/`.xcworkspace` are produced.

**Not yet covered** — the "CocoaPods compatible" claim in ARCHITECTURE.md §6 still requires fixtures for: transitive dependencies, subspecs, git dependencies, XCFrameworks, resource bundles, multi-target projects, and an existing hand-authored `Podfile.lock`. Current coverage is one local-path dependency fixture, chosen deliberately because it's hermetic (no network, no real Xcode.app build) — broader coverage, especially git-based deps, needs network access in CI and is the next increment within this milestone, not a new one. `engines/cocoapods/podfile`/`podspec` parsing (via `pod ipc`) for federation use is deferred to Milestone 5, since Phase 1 delegation doesn't need it yet.

## Milestone 3 — Public registry
`registry-server`: metadata API, versioning, search, storage, auth. No publishing from the CLI yet — server-side only, validated via direct API tests.

## Milestone 4 — Publishing
`zorvok login`, `zorvok trunk push`. Publish a real test podspec end-to-end. `internal/publisher` + server-side validation (ADR 0003 constraints: immutability, ownership, duplicate-version rejection).

## Milestone 5 — Registry federation
`zorvok install` resolves from both Zorvok and CocoaPods registries using the precedence rules in ADR 0003. Dependency-confusion adversarial review required before this ships.

## Milestone 6 — Independent installation engine
Phase 3 of ADR 0002: resolution moves from delegated `pod install` to a narrower Ruby shim calling `cocoapods-core` + `molinillo` + `xcodeproj` directly, under Zorvok's federation authority. Ships behind a flag with differential testing against the delegated path before becoming default. Xcode project generation remains on `xcodeproj` — not reimplemented, ever (ADR 0002).

## Milestone 7 — Public launch
CLI + Homebrew tap + website + documentation + public registry, all versioned and backward-compatible per the engineering rules below.

---

## Engineering rules (apply to every milestone)
- No Android/Maven/Gradle functionality.
- No forced SPM migration.
- Never break an existing, unmodified CocoaPods project.
- No large unvalidated code drops — each milestone ships behind its own acceptance criteria.
- Automated tests required per milestone (see `tests/`).
- Docs (`ARCHITECTURE.md`, ADRs, this roadmap, `CHANGELOG.md`) kept current as decisions change.
