# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**This repository is the CLI.** The registry lives in two sibling repositories, per [ADR 0009](docs/adr/0009-three-tier-registry-model.md)'s three-tier model:

- [zorvok-dev/zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios) — a self-hostable registry server, for private/enterprise use (Tier 3) — same role Nexus/Artifactory play for Maven.
- [zorvok-dev/zorvok-specs](https://github.com/zorvok-dev/zorvok-specs) — Zorvok's own free public registry: static files served via GitHub Pages, no server, no hosting cost to the project (Tier 2).
- CocoaPods' own registry (`cdn.cocoapods.org`) is Tier 1, unchanged.

All three are reached the exact same way — `zorvok install --native --source <url>`, or automatically once `registry_url` is configured (ADR 0008) — `zorvok install` doesn't care which kind of thing answers at that URL.

**Status: Milestones 1-4 complete; Milestone 6 (native installer + automatic registry federation) works end-to-end, proven against both a real running registry server and a pure static file server.** CLI: `zorvok --version`, `--help`, `init`, `config`, `doctor`, `install`, `update`, `lib lint`, `login`, `trunk push`, `search`, `info` are implemented and tested. `install`/`update`/`lib lint` delegate to a real local CocoaPods install per [ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md), verified against 9 real compatibility fixtures. `login`/`trunk push` publish to a real Zorvok registry server (zorvok-registry): register → verify → lint → publish → immutability-enforced. `zorvok install --native` ([ADR 0005](docs/adr/0005-native-installer-and-federation-mechanism.md), [ADR 0006](docs/adr/0006-cdn-facade.md), [ADR 0008](docs/adr/0008-automatic-federation.md)) drives `Pod::Installer` directly; once `registry_url` is configured, it **automatically** resolves dependencies that appear **nowhere in the Podfile** — without ever writing to it — whether `registry_url` points at a live zorvok-registry or a plain static file server (verified against both). Federation safety was taken seriously before automating it, not assumed: [ADR 0007](docs/adr/0007-federation-safety-fixes.md) reproduced a real dependency-confusion gap directly against CocoaPods (two sources with the same pod silently resolve by declaration order, exit 0) and fixed it, and it even caught a real, unplanned collision during this session's own testing — `--native` now hard-refuses an ambiguous, unpinned pod before downloading anything. `registry *` is the only command left with no CLI wiring. **Known gaps, not silently assumed away:** no PostgreSQL backend for zorvok-registry (in-memory only), no real email delivery (verification links are logged, not emailed), `zorvok trunk push` doesn't yet know how to publish to the static zorvok-dev/zorvok-specs registry (PR-based publishing only, for now), no reserved-name/publisher-reputation policy beyond "verified email, first publisher wins" / GitHub-PR ownership — see [ROADMAP.md](ROADMAP.md) for the full picture.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

## Try it now (Milestones 1-6)

```bash
go build -o zorvok ./cmd/zorvok   # Homebrew distribution lands later

./zorvok doctor     # check your local Ruby/CocoaPods/git environment
./zorvok init       # create .zorvok.yml in a project (never touches your Podfile)
./zorvok config set email you@example.com

cd /path/to/an/existing/CocoaPods/project
/path/to/zorvok install   # delegates to `pod install` (ADR 0002, Phase 1)
/path/to/zorvok update
/path/to/zorvok lib lint

# Milestone 6: Zorvok's own installer, opt-in. With no --source this is
# behaviorally identical to the line above (verified by a differential
# test); --source resolves a dependency that isn't declared in the
# Podfile at all, without ever writing to it (ADR 0005).
/path/to/zorvok install --native
/path/to/zorvok install --native --source https://your-cocoapods-specs-repo.example/
```

Registry server + publishing, end to end (clone [zorvok-dev/zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios) alongside this repo):

```bash
cd ../zorvok-registry && go build -o zorvok-registry ./cmd/zorvok-registry && cd -
../zorvok-registry/zorvok-registry   # in-memory only — all data is lost on restart

# in another terminal:
zorvok config set registry_url http://localhost:8080 --global
zorvok login you@example.com
# verification link is logged to the server's own output (no real email transport yet) —
# copy it and open it (or curl it) to verify before you can publish

cd /path/to/a/pod/with/a/single/MyPod.podspec
zorvok trunk push              # lints, converts via `pod ipc spec`, and publishes
curl localhost:8080/v1/packages/MyPod
curl localhost:8080/v1/search?q=MyPod

# now consume it from a different project, with zero Podfile awareness
# of where MyPod comes from — registry_url is already configured
# (set above), so this federates automatically (ADR 0008), no --source
# flag needed:
cd /path/to/a/different/project   # Podfile just says: pod 'MyPod', '1.0.0'
zorvok install --native
# zorvok install --native --no-registry   # opt out of auto-federation for one run
```

## Running the test suite

```bash
go test ./...
```

Most packages are self-contained, but `internal/registry` and `internal/publisher`'s integration tests exercise a real registry — they build and run a real `zorvok-registry` subprocess rather than importing its Go packages directly (those now live in a separate repo, ADR 0009). Build it from [zorvok-dev/zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios) and put the binary on `PATH` first, or those tests skip (not fail):

```bash
cd ../zorvok-registry && go build -o /usr/local/bin/zorvok-registry ./cmd/zorvok-registry
```

`engines/cocoapods/compatibility` and others similarly skip without a local `pod`/`git` install.

## Not yet implemented

`brew install zorvok` — the formula exists and is verified ([zorvok-dev/homebrew-tap](https://github.com/zorvok-dev/homebrew-tap)), but can't actually work until `zorvok-ios` is pushed and tagged (see that tap repo's README) — along with `zorvok registry list`/`registry add` and `zorvok trunk push` targeting the static zorvok-dev/zorvok-specs registry (PR-based publishing works today; see that repo's own README).

## Scope

Zorvok is iOS-only. No Android, Maven, or Gradle functionality is in scope.

## Contributing

Not yet open for contribution — architecture is pending approval (see [ROADMAP.md](ROADMAP.md)). `CONTRIBUTING.md` will be added once Milestone 1 begins.
