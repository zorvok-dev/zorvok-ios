# Zorvok Roadmap

Status key: ✅ done · 🚧 in progress · ⬜ not started

## Milestone 0 — Architecture ✅
Researched CocoaPods' internals (CLI, CocoaPods-Core, Molinillo, cocoapods-downloader, xcodeproj, Trunk, CDN, Podfile.lock format), chose Go for the CLI ([ADR 0001](docs/adr/0001-cli-language-go.md)), and settled the delegation ([ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md)) and registry ([ADR 0003](docs/adr/0003-registry-federation-and-storage.md)) strategies. Architecture approved in [ARCHITECTURE.md](ARCHITECTURE.md).

## Milestone 1 — CLI skeleton ✅
`zorvok --version`, `--help`, `doctor`, `config`, `init`. Everything else started as a named, documented "not implemented yet" command rather than an unknown one — all of it is implemented now (see below). Go, stdlib-only, unit tested. The module path `github.com/zorvok-dev/zorvok-ios` was a placeholder that turned out to match a real GitHub org.

## Milestone 2 — CocoaPods compatibility 🚧
`zorvok install`, `zorvok update`, and `zorvok lib lint` delegate to real `pod install`/`pod update`/`pod lib lint` (ADR 0002). Compatibility tests run real, unmodified CocoaPods end to end — no mocks, hermetic fixtures (local-path and `file://` git repos, no network, no Xcode.app).

| Surface | Fixture | Status |
|---|---|---|
| Simple Podfile | `local-pod` + `simple-project` | ✅ |
| Multiple / transitive dependencies | `transitive` (positive and negative case) | ✅ |
| Subspecs | `subspecs` | ✅ |
| Git dependencies | `git-dependency` | ✅ |
| Resource bundles | `resource-bundle` | ✅ |
| Multiple Xcode targets | `multi-target` | ✅ |
| Reinstall is idempotent | `simple-project` | ✅ |
| Local path dependencies | every fixture above | ✅ |
| XCFrameworks | `xcframework`, built with `clang` + `xcodebuild -create-xcframework` | ✅ |
| `update` moves forward, `install` stays locked | `updatable-pod` + `updatable-project` | ✅ |
| Importing a hand-authored `Podfile.lock` from elsewhere | — | ⬜ not yet built |

Podfile/podspec parsing for federation (via `pod ipc`) was deferred to Milestone 6, where it's actually needed.

## Milestone 3 — Public registry 🚧
The registry server exposes a versioned HTTP API: package/version metadata with ownership and immutability, Trunk-style email-verified sessions. `search`/`info` and `login`/`trunk push` all talk to it.

What's tested, not just built: ownership (a non-owner publishing to someone else's name is rejected, 403), immutability (republishing a version is rejected, 409, even with identical content), auth gating (publishing requires a verified session), no server-side code execution (a podspec is validated structurally and stored as submitted, never parsed as Ruby), and a per-IP rate limiter.

Known gaps: no PostgreSQL backend yet — the schema is designed per ADR 0003, but only the in-memory store is wired up, so registry data doesn't survive a restart. No real email transport; verification links are logged, not sent. No artifact storage — a podspec's `source` is a reference URL, which is the intended scope, not a shortcut.

## Milestone 4 — Publishing ✅
`zorvok login` and `zorvok trunk push` are wired to a real registry — tested against a real httptest server and, separately, by hand against a running `zorvok-registry` process with the compiled binary: register, verify, push, published, a re-push correctly rejected.

`internal/registry` is an HTTP client tested against a real server, not a mock. `internal/publisher` finds the one podspec in the current directory (an explicit error on zero or more than one), lints it for real before publishing, and refuses to publish a podspec whose source is a local path, since nobody else could install from it. Sessions are stored per registry host; `registry_url` defaults to a local dev server, never a hardcoded production URL.

Not yet done: no real email transport (same gap as Milestone 3), and publishing only ever reaches one registry — no federation yet.

## Milestone 5 — Registry federation, re-scoped 🚧
The original plan was automatic federation with zero Podfile awareness. That turned out not to be possible while still delegating to `pod install`: it has no flag to add a source for a single run, and a globally registered spec repo isn't implicitly searched unless the Podfile names it — confirmed against real CocoaPods' own error message, not assumed ([ADR 0004](docs/adr/0004-federation-requires-podfile-source-line.md)). A source has to be declared somewhere.

What shipped instead: using a Zorvok registry is exactly as opt-in as using any other private CocoaPods source — a `source 'https://...'` line. `zorvok registry list/add/add-cdn/remove/update` wrap `pod repo` for managing those sources, verified against this machine's real CocoaPods repos. True zero-Podfile-touch federation moved to Milestone 6, where it turned out to be achievable after all.

Still open:
- ⬜ A `zorvok doctor` check that flags when a configured registry isn't actually present as a Podfile `source` line.
- ⬜ Written docs for this opt-in workflow, beyond what's here and in the README.

## Milestone 6 — Native installation engine 🚧
`zorvok install --native` drives `Pod::Installer` directly — the same class `pod install` itself uses, so Molinillo still resolves and `xcodeproj` still generates the project. It supplies an in-memory, augmented Podfile instead of letting CocoaPods reread the file, which is what lets a federated source get injected without the Podfile on disk ever being touched ([ADR 0005](docs/adr/0005-native-installer-and-federation-mechanism.md)).

Four pieces, each tested against real CocoaPods:
- **The installer itself.** With no `--source`, it produces a byte-identical `Podfile.lock` to the delegated path — the bar ADR 0002 sets before this can ever become default. With `--source`, it resolves a pod declared nowhere in the Podfile, and the Podfile's bytes are unchanged before and after.
- **A CDN facade** ([ADR 0006](docs/adr/0006-cdn-facade.md)) so a live Zorvok registry can be used as a real CocoaPods CDN source, not just a local fixture — confirmed end to end: a running registry, a real `trunk push`, and `zorvok install --native --source` installing the published pod.
- **Two bugs found while building the above, fixed before continuing** ([ADR 0007](docs/adr/0007-federation-safety-fixes.md)): injecting a federated source silently dropped the Podfile's implicit default CDN, breaking every pod with no explicit source of its own; and two sources offering the same pod/version resolve by declaration order with only a warning in real CocoaPods, not a hard failure. Both are fixed — the second is now a hard refusal.
- **Automatic federation** ([ADR 0008](docs/adr/0008-automatic-federation.md)). With no `--source` flags, `--native` adds the configured `registry_url` automatically, but only when a user actually configured one; `--no-registry` opts out per run. The ADR 0007 hard-refuse is what makes this safe to automate.

Still open: `--native` stays opt-in, not default. The registry has no reserved-name or publisher-reputation system beyond first-publisher-wins — a known, accepted gap at the registry's moderation layer, not something the installer can fix. The CDN facade computes responses from the in-memory store on every request rather than precomputed index files; fine at this scale, worth revisiting with real traffic. `pod repo update` against a Zorvok CDN source and deprecated-pod signaling aren't implemented. `lib lint`/`trunk push` still delegate to `pod` directly — only `install` has a native path.

## Milestone 7 — Public launch 🚧
CLI, Homebrew tap, documentation, and public registry — versioned and backward-compatible.

**Command parity with CocoaPods: done.** Every command in `pod --help` has a Zorvok equivalent — the registry-native `trunk` family (`register`/`me`/`push`/`add-owner`/`remove-owner`/`deprecate`/`info`; `delete` refuses and points at `deprecate`, since published versions are immutable by design), plus generic delegation for every pass-through command (`spec`, `cache`, `ipc`, `plugins`, `list`, `outdated`, `try`, `env`, `setup`, `deintegrate`, `lib create`, `registry lint`/`push`). See the README's command reference.

**Homebrew: built, not yet publishable.** The formula passes `brew install`/`test`/`audit --strict`/`style`. It can't resolve until `zorvok-ios` has a tagged release for its `url`/`sha256` to point at — see the tap repo's README for the release steps. In the meantime, `go install github.com/zorvok-dev/zorvok-ios/cmd/zorvok@main` already works.

## Milestone 9 — Three-tier registry split ✅
The registry became three tiers instead of one server: CocoaPods' own, a free static public registry, and a self-hostable server — the Maven model, a public index plus an optional self-hosted Nexus/Artifactory equivalent ([ADR 0009](docs/adr/0009-three-tier-registry-model.md)).

- This repo kept the CLI only; the registry server moved to its own repo (`zorvok-registry-ios`), same architecture, own `go.mod`.
- `zorvok-specs` is a new, static public registry: the same CDN protocol the live server speaks, as pre-committed files. Ownership and immutability, normally a database's job, are enforced by a GitHub Actions PR check instead — tested locally against real attack scenarios (ownership hijack, immutability violation, name/version mismatch, malformed podspec) before being wired into the workflow.
- This repo's integration tests now run a real `zorvok-registry` binary as a subprocess instead of importing its Go packages directly, since that's no longer possible across repos.

Confirmed with a plain static file server standing in for GitHub Pages: `zorvok install --native --source` correctly detected it as a CDN source and installed from it, no server-side code involved. A test pod name happened to collide with a real public CocoaPods pod along the way — `zorvok install` correctly refused to resolve it rather than silently picking a source, an unplanned but useful confirmation of the ADR 0007 guard.

Not done: `trunk push` still only publishes to a live registry server, not to `zorvok-specs` — PR-based publishing only, for now. The GitHub Actions workflow YAML hasn't run against real GitHub Actions yet; only the scripts it calls are tested locally. GitHub Pages isn't enabled for that repo yet either, so `raw.githubusercontent.com` serves the same files in the meantime.

## Publishing this repository
`zorvok-ios` and `zorvok-specs` are pushed to GitHub and public. `zorvok-registry-ios` is pushed and kept private, since it holds the registry's internal implementation. `homebrew-tap` exists locally, built and verified, but hasn't been pushed yet — see Milestone 7.

---

## Engineering rules (apply to every milestone)
- No Android/Maven/Gradle functionality.
- No forced SPM migration.
- Never break an existing, unmodified CocoaPods project.
- No large unvalidated code drops — each milestone ships behind its own acceptance criteria.
- Automated tests required per milestone (see `tests/`).
- Docs (`ARCHITECTURE.md`, ADRs, this roadmap) kept current as decisions change.
