# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**Status: Milestones 1-4 complete; Milestone 6 (native installer + automatic registry federation) works end-to-end against a real running Zorvok registry — see what's still missing below.** CLI: `zorvok --version`, `--help`, `init`, `config`, `doctor`, `install`, `update`, `lib lint`, `login`, `trunk push` are implemented and tested. `install`/`update`/`lib lint` delegate to a real local CocoaPods install per [ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md), verified against 9 real compatibility fixtures. `login`/`trunk push` publish to a real Zorvok registry (`cmd/zorvok-registry`): register → verify → lint → publish → immutability-enforced. `zorvok install --native` ([ADR 0005](docs/adr/0005-native-installer-and-federation-mechanism.md), [ADR 0006](docs/adr/0006-cdn-facade.md), [ADR 0008](docs/adr/0008-automatic-federation.md)) drives `Pod::Installer` directly; once `registry_url` is configured, it **automatically** resolves dependencies published to the Zorvok registry that appear **nowhere in the Podfile** — no `--source` flag needed — without ever writing to it. Federation safety was taken seriously before automating it, not assumed: [ADR 0007](docs/adr/0007-federation-safety-fixes.md) reproduced a real dependency-confusion gap directly against CocoaPods (two sources with the same pod silently resolve by declaration order, exit 0) and fixed it — `--native` now hard-refuses an ambiguous, unpinned pod before downloading anything, automatic or not — and [ADR 0008](docs/adr/0008-automatic-federation.md)'s adversarial review worked through the attack scenarios before automatic federation shipped at all. Proven with a full real-stack run: a real `cmd/zorvok-registry`, `registry_url` configured once, a real `trunk push`, then `zorvok install --native` with **zero flags** — resolved the pod, left the Podfile byte-unchanged, and `--no-registry` was confirmed to correctly opt back out. `search`, `info`, and `registry *` have no CLI wiring yet. **Known gaps, not silently assumed away:** no PostgreSQL backend (in-memory only), no real email delivery (verification links are logged, not emailed), no reserved-name/publisher-reputation policy on the registry beyond "verified email, first publisher wins" — see [ROADMAP.md](ROADMAP.md) for the full picture.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

## Try it now (Milestones 1-4)

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

Registry server + publishing, end to end:

```bash
go build -o zorvok-registry ./cmd/zorvok-registry
./zorvok-registry   # in-memory only — all data is lost on restart, see ROADMAP.md

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

## Intended developer experience (not yet implemented)

```bash
brew install zorvok

cd MyExistingiOSProject
zorvok install        # drop-in replacement for `pod install` — no Podfile changes required

zorvok search Alamofire
zorvok info Alamofire

zorvok login
zorvok lib lint
zorvok trunk push      # publish to the independent Zorvok registry
```

## Scope

Zorvok is iOS-only. No Android, Maven, or Gradle functionality is in scope.

## Contributing

Not yet open for contribution — architecture is pending approval (see [ROADMAP.md](ROADMAP.md)). `CONTRIBUTING.md` will be added once Milestone 1 begins.
