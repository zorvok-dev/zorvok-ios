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
`zorvok --version`, `--help`, `doctor`, `config`, `init`. No CocoaPods interaction yet — `install`/`update`/`search`/`info`/`lib lint`/`login`/`trunk push`/`registry *` all exist as named, documented, explicitly "not implemented yet" commands rather than failing as unknown or silently no-op'ing. Built in Go (module `github.com/zorvok-dev/zorvok-ios` — chosen as a placeholder at the time, later confirmed to match a real `zorvok-dev` GitHub org; see "Publishing this repository" below), stdlib-only, unit tested.

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
`registry-server` implements a versioned HTTP API (`registry-server/api`), package/version metadata with ownership and immutability (`registry-server/packages`), and Trunk-style email-verified session auth (`registry-server/auth`). `cmd/zorvok-registry` runs it; verified end-to-end both via `go test ./registry-server/...` (httptest) and a real running server exercised with `curl` (register → verify → publish → get → duplicate-rejected). CLI wiring landed later, across Milestones 4 (`login`/`trunk push`) and this increment (`search`/`info`, via `internal/registry.Client.Search`) — all four talk to the same registry API.

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

## Milestone 4 — Publishing ✅
`zorvok login <email>` and `zorvok trunk push [podspec] [--allow-warnings] [--skip-lint]` are implemented and wired to a real registry. Verified both ways: `go test ./internal/publisher/...` runs the full flow (real `pod lib lint` + real `pod ipc spec` + a real httptest registry server) including negative cases, and I also drove it manually against a real running `cmd/zorvok-registry` with the actual `zorvok` binary — register → verify → `trunk push` → published → re-push correctly rejected with 409.

What's covered, with real tests, not just described:
- **`internal/registry`**: a Go HTTP client for the registry API, tested against a real server (not a mock) — register, publish, get, and each error path (404/403/409) as a typed `*registry.APIError`.
- **`internal/publisher`**: locates the single `*.podspec` in the current directory (explicit error if zero or more than one — never guesses), lints it with real `pod lib lint` before publishing anything, converts it to JSON with real `pod ipc spec`, and **rejects client-side** a podspec whose `source` is a local `:path` (useless to publish — nobody else could install it) before any network call. `TestPushFailsLintBeforePublishing` confirms a podspec that fails lint never reaches the registry at all.
- **`zorvok login`**: stores the session token via `internal/auth`'s per-registry-host credential store (built in Milestone 1), keyed by the configured `registry_url`.
- **Config**: `registry_url` (project-over-global, like every other `zorvok config` key) defaults to `http://localhost:8080` — the local dev server's own default — since there's no public Zorvok registry yet; never a hardcoded production URL.

**Not yet done**: no real email transport (same gap as Milestone 3 — `zorvok login` prints the same guidance about checking the dev server's log), and `zorvok trunk push` talks to exactly one registry (no federation — that's Milestone 5).

## Milestone 5 — Registry federation (re-scoped) ⚠️
**Re-scoped mid-milestone by [ADR 0004](docs/adr/0004-federation-requires-podfile-source-line.md).** The original plan — `zorvok install` automatically resolving from both registries using the precedence rules in ADR 0003, with zero Podfile awareness of Zorvok — turned out not to be achievable while still delegating to `pod install` (ADR 0002, Phase 1-2). This was discovered by testing the actual mechanism before building on it, not assumed: `pod install` has no flag to inject an extra source, and a spec repo registered globally via `pod repo add` is not implicitly searched — confirmed with CocoaPods' own error message, *"not added the source repo that hosts the Podspec to your Podfile."* A source must be declared in the Podfile itself; there is no invisible way to add one for a single run.

**What Milestone 5 delivers instead**: using the Zorvok registry is opt-in, via the same one-line `source 'https://...'` a developer would add for any CocoaPods private registry — not a Zorvok-specific migration step. Remaining scope:
- ⬜ Document the opt-in `source` line workflow (install guide / website docs, Milestone 6).
- ⬜ `zorvok registry add <url>` / `zorvok registry list` — thin, documented wrappers around `pod repo add`/`pod repo list` with Zorvok-specific guidance (still listed as not-yet-implemented in `zorvok help`).
- ⬜ `zorvok doctor` check for whether the configured Zorvok registry is present as a Podfile `source` line, to catch the easy "forgot to add the source" mistake with a clear message.

**True automatic, zero-Podfile-touch federation — the original Milestone 5 goal — moves to Milestone 6**, where it's actually achievable: once Zorvok's own resolver controls the source list programmatically, the precedence rules in ADR 0003/ARCHITECTURE.md §2.3 apply for real, and the dependency-confusion adversarial review happens before *that* ships.

## Milestone 6 — Independent installation engine 🚧
Phase 3 of ADR 0002, first increment done: **[ADR 0005](docs/adr/0005-native-installer-and-federation-mechanism.md)**. `zorvok install --native [--source <url> ...]` drives `Pod::Installer` directly (the same class `pod install` itself uses — Molinillo still resolves, `xcodeproj` still generates the project, nothing reimplemented), supplying an in-memory-augmented Podfile instead of letting CocoaPods read the file a second time. This is the resolution to ADR 0004's blocker: a federated source is injected without ever writing to the Podfile on disk.

Verified, not just described:
- `TestInstallNativeMatchesDelegatedInstall`: with no `--source`, the native path produces a **byte-identical** `Podfile.lock` to the delegated path — the differential-testing requirement ADR 0002 sets as the bar before this can ever become default.
- `TestInstallNativeFederatesUndeclaredDependency`: a pod named nowhere in the Podfile, sourced from nowhere the Podfile names, resolves via an injected `--source` — and the Podfile's bytes on disk are confirmed unchanged before/after.
- Both run fully isolated (`CP_HOME_DIR` pointed at a temp dir — a CocoaPods-native override, not a Zorvok one) with no network access and no shared state with a real machine's `~/.cocoapods`; the same flow was also run by hand through the compiled `zorvok` binary against this machine's real CocoaPods install.
- `engines/cocoapods/adapter`'s Ruby-environment resolution (finding the right `ruby` interpreter + `GEM_HOME` from wherever `pod` itself is installed — Homebrew's isolated-Cellar shape and a plain `gem install cocoapods` shape are both handled, unit tested with synthetic fixtures) is its own small, necessary piece of portability work this needed.

**Second increment done: [ADR 0006](docs/adr/0006-cdn-facade.md)** — `registry-server/cdn` serves the real CocoaPods CDN source protocol (read directly from the `cocoapods`/`cocoapods-core` gem source before writing any code, same discipline as ADR 0004/0005) in front of `registry-server/packages`, mounted at `/cdn/` on `cmd/zorvok-registry`. This closes the gap the first increment left open: `zorvok install --native --source <zorvok-registry-url>/cdn/` now works against a **real, running Zorvok registry**, not just a hand-built local Specs repo fixture.

Verified with a full real-stack proof, not just Go tests: started a real `cmd/zorvok-registry`, published a real podspec to it via the real `zorvok trunk push` (pointed at a real, separately-hosted git repo as the pod's actual source — a full install really does fetch `:source`, unlike `lib lint`), then ran the compiled `zorvok` binary's `install --native --source http://localhost:8099/cdn/` against a Podfile that names the pod by name and version only. CocoaPods' own log confirmed it recognized the registry as a real CDN source (`Adding spec repo ... with CDN ...`), resolved and installed the pod, and `Podfile.lock` recorded `SPEC REPOS: http://localhost:8099/cdn/: - CDNFedPod` — provenance tracking for free. The Podfile on disk was confirmed byte-unchanged. `registry-server/cdn` also has its own httptest coverage per endpoint, including the two negative cases that matter (a request shape CDNSource would never construct, and a pod not on this registry — both 404 cleanly, the second being exactly ADR 0003's federation fallthrough case).

**Third increment: [ADR 0007](docs/adr/0007-federation-safety-fixes.md)** — two bugs found in ADR 0005/0006 while investigating what "automatic federation" would require, fixed before continuing. One functional (injecting a federated source silently dropped the implicit default CDN, breaking every ordinary pod in a Podfile with no `source` lines of its own — every prior test only ever exercised a Podfile with one, fully-federated pod, so this went uncaught until a mixed-pod scenario was deliberately tested). One security (confirmed by reproducing it against real CocoaPods: two sources with the same pod+version resolve by *declaration order* with only a warning, exit code 0 — not a hard failure, and exploitable the instant Zorvok's injected source and any other source both have the same pod name). Both fixed and covered by dedicated tests: `TestInstallNativePreservesImplicitDefaultSource`, `TestInstallNativeAbortsOnAmbiguousSource`, `TestInstallNativePinnedSourceBypassesAmbiguityCheck`.

**Fourth increment: [ADR 0008](docs/adr/0008-automatic-federation.md) — automatic federation, with the adversarial review done first.** `zorvok install --native` with no `--source` flags now automatically adds the configured `registry_url` as a source — but *only* when a user has actually run `zorvok config set registry_url ...` (never the bundled `http://localhost:8080` fallback nobody configured). `--no-registry` opts back out per invocation. The review itself worked through concrete attack scenarios before anything was built: a name-collision attack is already covered by ADR 0007's hard-refuse; a no-collision attack (an unclaimed name resolving silently) is real but not novel to federation — it's the same exposure any single open registry already has, mitigated at the registry's ownership/moderation layer, not the installer; and ADR 0003's original "Zorvok-owned names always win ties" rule is formally superseded by ADR 0007's stricter hard-refuse-on-ambiguity, which remains the permanent policy, not an interim one. Verified with a full real-stack proof: a real `cmd/zorvok-registry`, `registry_url` configured via `zorvok config set --global`, a real `trunk push`, then `zorvok install --native` with **zero flags** — it automatically added both the Zorvok registry and (since the Podfile had no source lines) the real `cdn.cocoapods.org`, resolved the published pod, and left the Podfile byte-unchanged. `--no-registry` was verified to correctly skip the registry and fail resolution as expected.

**Still open, explicitly**:
- `--native` is opt-in, not default (ADR 0002's own requirement) — `zorvok install` without `--native` is unchanged, still the Milestone 2 delegated path.
- ADR 0003's ownership-based precedence ("a Zorvok-owned name always wins") is explicitly **not** implemented and, per ADR 0008, never will be in that form — hard-refuse-on-ambiguity replaces it as the permanent safety policy. Provenance beyond what CocoaPods' own `SPEC REPOS` section already gives for free isn't implemented either.
- The registry itself still has no reserved-name policy or publisher-reputation system beyond "verified email, first publisher wins" (M3) — accepted, pre-existing, registry-moderation-layer risk per ADR 0008's review, not something the installer can or should fix.
- `registry-server/cdn` computes its responses on the fly from the in-memory store on every request, not from precomputed static index files the way ARCHITECTURE.md §2.4 originally envisioned (modeled on `cdn.cocoapods.org`'s own read-heavy optimization) — fine at today's scale, revisit once there's a real backend and real traffic.
- `pod repo update` against a Zorvok CDN source and deprecated-pod signaling (`deprecated_podspecs.txt`) aren't implemented.
- `pod lib lint`/`pod ipc spec` (used by `zorvok lib lint`/`trunk push`) still delegate to the `pod` CLI binary, unchanged — only `install` has a native path so far.

## Milestone 7 — Public launch
CLI + Homebrew tap + website + documentation + public registry, all versioned and backward-compatible per the engineering rules below.

## Milestone 9 — Three-tier registry split ✅ (local), not yet pushed
**[ADR 0009](docs/adr/0009-three-tier-registry-model.md)**: the registry became three tiers instead of one server — CocoaPods' own, Zorvok's free static public registry, and a self-hostable server for private/enterprise use (Maven-style: public registry + self-hosted Nexus/Artifactory-equivalent). Concretely:

- **This repo** kept the CLI only — `registry-server/` and `cmd/zorvok-registry/` were removed.
- **`zorvok-dev/zorvok-registry-ios`** (new, separate repo, own `go.mod`): everything that was `registry-server/` + `cmd/zorvok-registry/`, unchanged architecture, builds and tests green standalone.
- **`zorvok-dev/zorvok-specs`** (new, separate repo): a static public registry — the exact same CDN protocol `zorvok-registry/cdn` serves live, as pre-committed files instead. Ownership/immutability, normally a database's job, are enforced by a GitHub Actions PR check (`scripts/validate-pr.sh`) instead — tested locally against real git repos and real attack scenarios (ownership hijack attempt, immutability violation, name/version mismatch, invalid JSON) before being wired into the workflow YAML, though the YAML itself hasn't run against real GitHub Actions yet (no Actions access available).
- This repo's own integration tests (`internal/registry`, `internal/publisher`) switched from importing `registry-server`'s Go packages directly (no longer possible — separate repo) to running a real `zorvok-registry` binary as a subprocess (`internal/registrytest`), the same pattern already used for testing against real `pod`/`git`.

**Verified, not just built**: ran a plain local static HTTP file server over the `specs` skeleton (standing in for GitHub Pages) and confirmed `zorvok install --native --source http://localhost:.../` correctly detected it as a real CocoaPods CDN source and installed a published pod from it — zero server-side code running. Along the way, an unplanned confirmation of ADR 0007's dependency-confusion guard: a test pod name happened to collide with a real, already-registered CocoaPods pod, and `zorvok install` correctly refused to resolve it rather than silently picking a source.

**Not done**:
- Nothing has been pushed to GitHub — this session has no `gh` CLI and no push credentials configured. All three repos exist locally, fully built and tested, ready to push.
- `zorvok trunk push` doesn't yet know how to publish to `zorvok-dev/zorvok-specs` (PR-based publishing only, for now) — only to a live `zorvok-registry` server (Milestone 4).
- The `zorvok-dev/zorvok-specs` GitHub Actions workflow YAML is unverified against real GitHub Actions — only the shell scripts it calls are tested.

## Publishing this repository
The Go module path chosen in Milestone 0 (`github.com/zorvok-dev/zorvok-ios`) was a placeholder at the time — it turned out to match a real GitHub organization (`zorvok-dev`). Three repos now exist locally (`zorvok`, `zorvok-registry`, `specs` — Milestone 9 above) and are ready to push, but haven't been: this session has neither `gh` installed nor push credentials configured. Either authenticate `gh` in this environment, or push manually.

---

## Engineering rules (apply to every milestone)
- No Android/Maven/Gradle functionality.
- No forced SPM migration.
- Never break an existing, unmodified CocoaPods project.
- No large unvalidated code drops — each milestone ships behind its own acceptance criteria.
- Automated tests required per milestone (see `tests/`).
- Docs (`ARCHITECTURE.md`, ADRs, this roadmap, `CHANGELOG.md`) kept current as decisions change.
