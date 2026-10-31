# Zorvok Roadmap

Status key: ✅ done · 🚧 in progress · ⬜ not started

## Milestone 0 — Architecture *(this milestone)*
- ✅ Research CocoaPods internals (CLI, CocoaPods-Core, Molinillo, cocoapods-downloader, xcodeproj, Trunk, CDN, Podfile.lock format)
- ✅ Select CLI language ([ADR 0001](docs/adr/0001-cli-language-go.md))
- ✅ Define CLI-to-engine interface ([ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md))
- ✅ Define registry architecture ([ADR 0003](docs/adr/0003-registry-federation-and-storage.md))
- ✅ Architecture document with component diagram, trade-offs, risks ([ARCHITECTURE.md](ARCHITECTURE.md))
- ⬜ **Architecture approval** — gate before Milestone 1 starts

## Milestone 1 — CLI skeleton
`zorvok --version`, `--help`, `doctor`, `config`, `init`. No CocoaPods interaction yet. Establishes `internal/cli`, `internal/config`, `internal/auth` foundations and the Homebrew-distributable binary shape.

## Milestone 2 — CocoaPods compatibility
`zorvok install`, `zorvok update`, `zorvok lib lint`. Delegates to real `pod install`/`pod update` (ADR 0002, Phase 1). `engines/cocoapods/{podfile,podspec,installer}` built here. Gated by the compatibility test suite in `tests/compatibility` (ARCHITECTURE.md §6) before any "CocoaPods compatible" claim.

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
