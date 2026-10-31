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

**Compatibility fixture coverage** (all run against real, installed CocoaPods — no mocking — and all hermetic: local-path and file:// git repos generated at test time, no network access, no real Xcode.app build, only the `xcodeproj` gem):

| Surface (brief §10) | Fixture | Status |
|---|---|---|
| Simple Podfile | `tests/fixtures/local-pod` + `simple-project` | ✅ |
| Multiple dependencies | `tests/fixtures/transitive` (positive case) | ✅ |
| Transitive dependencies | `tests/fixtures/transitive` — both the positive case and a negative case proving an undeclared local dependency fails resolution rather than being silently skipped | ✅ |
| Subspecs | `tests/fixtures/subspecs` — verifies the undeclared subspec's files never reach the generated Pods project | ✅ |
| Git dependencies | `tests/fixtures/git-dependency` — repo generated and tagged at test time, installed via `file://` | ✅ |
| Resource bundles | `tests/fixtures/resource-bundle` — verifies a real `com.apple.product-type.bundle` target is generated | ✅ |
| Multiple Xcode targets | `tests/fixtures/multi-target` — verifies per-target `Pods-<Target>` integration | ✅ |
| Existing `Podfile.lock` (no drift on reinstall) | reuses `simple-project` | ✅ (`TestReinstallIsIdempotent`) |
| Local path dependencies | covered by every fixture above | ✅ |
| Binary frameworks / XCFrameworks | `tests/fixtures/xcframework` — a real `.xcframework` built with `clang` + `xcodebuild -create-xcframework` (no full Xcode project needed); verifies the host app target gets wired to `PODS_XCFRAMEWORKS_BUILD_DIR` | ✅ |
| An existing hand-authored `Podfile.lock` from a *different* CocoaPods version/project | — | ⬜ not yet built |

`engines/cocoapods/podfile`/`podspec` parsing (via `pod ipc`) for federation use is deferred to Milestone 5, since Phase 1 delegation doesn't need it yet.

## Milestone 3 — Public registry 🚧
`registry-server` implements a versioned HTTP API (`registry-server/api`), package/version metadata with ownership and immutability (`registry-server/packages`), and Trunk-style email-verified session auth (`registry-server/auth`) — server-side only, no CLI wiring yet (that's Milestone 4). `cmd/zorvok-registry` runs it; verified end-to-end both via `go test ./registry-server/...` (httptest) and a real running server exercised with `curl` (register → verify → publish → get → duplicate-rejected).

What's covered, with real tests, not just described:
- **Ownership / package-takeover protection** (brief §9): first publisher of a name owns it; a different account publishing to that name is rejected (403) — `TestPublishRejectsNonOwner`.
- **Immutability** (brief §9, ADR 0003): republishing an existing `(package, version)` is rejected (409), even with different content — `TestPublishRejectsDuplicateImmutably`.
- **Auth gating**: publishing requires a verified session; an unverified account is rejected (403) even with a valid token — `TestPublishRejectsUnverifiedAccount`.
- **No server-side code execution** (brief §9): a submitted podspec is validated structurally (must be JSON, `name`/`version` must match the request) and stored verbatim — never parsed as Ruby or executed.
- **Rate limiting** (brief §9): a minimal per-IP fixed-window limiter, tested.

**Not yet done, and explicitly not assumed**:
- **PostgreSQL backend** — ADR 0003 calls for it; `registry-server/storage/postgres/schema.sql` documents the intended schema (with the DB-level `UNIQUE(package_name, version)` constraint ARCHITECTURE.md §2.4 calls for), but there's no `database/sql` implementation yet, and the schema itself is untested against a real database — this session's Docker daemon wasn't running. Only the in-memory store is implemented and used by `cmd/zorvok-registry` today; **all registry data is lost on restart**. See `registry-server/storage/README.md`.
- **Real email delivery** — `auth.LogSender` logs verification links instead of sending them; a real provider is a drop-in `auth.EmailSender` implementation later, not a design change.
- **Object storage / CDN for artifacts** — not needed yet; Milestone 3 is reference-only hosting (a podspec's `source` is a `:git`/`:http` URL, per ADR 0003's MVP scope).
- **Ownership-precedence adversarial review** (ARCHITECTURE.md §5.4) — still required before Milestone 5 (federation) ships; Milestone 3's ownership model (first-publisher-wins) is necessary but not sufficient for that review.

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
