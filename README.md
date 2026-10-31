# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**Status: Milestone 2 complete, Milestone 3 in progress.** CLI: `zorvok --version`, `--help`, `init`, `config`, `doctor`, `install`, `update`, `lib lint` are implemented and tested, delegating `install`/`update`/`lib lint` to a real local CocoaPods install per [ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md) — verified against 9 real compatibility fixtures (simple, transitive, subspecs, git, resource bundles, multi-target, XCFrameworks). Registry: `cmd/zorvok-registry` runs a working HTTP API (package/version metadata, ownership, immutability, Trunk-style email-verified auth) backed by an in-memory store — see [ROADMAP.md](ROADMAP.md) Milestone 3 for what's covered and what isn't (notably: no PostgreSQL backend yet, no real email delivery, all data lost on restart). `search`, `info`, `login`, `trunk push`, and `registry *` have no CLI wiring yet — running them prints an explicit "not implemented yet" message rather than failing silently or doing nothing.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

## Try it now (Milestone 1-3)

```bash
go build -o zorvok ./cmd/zorvok   # Homebrew distribution lands later

./zorvok doctor     # check your local Ruby/CocoaPods/git environment
./zorvok init       # create .zorvok.yml in a project (never touches your Podfile)
./zorvok config set email you@example.com

cd /path/to/an/existing/CocoaPods/project
/path/to/zorvok install   # delegates to `pod install` (ADR 0002, Phase 1)
/path/to/zorvok update
/path/to/zorvok lib lint
```

Registry server (no CLI wiring yet — `zorvok login`/`trunk push` are Milestone 4; talk to it directly):

```bash
go build -o zorvok-registry ./cmd/zorvok-registry
./zorvok-registry   # in-memory only — all data is lost on restart, see ROADMAP.md

curl -X POST localhost:8080/v1/auth/register -d '{"email":"you@example.com"}'
# verification link is logged to the server's own output (no real email transport yet)
curl localhost:8080/v1/auth/verify?token=...

curl -X POST localhost:8080/v1/packages/MyPod/versions \
  -H "Authorization: Bearer <session_token>" \
  -d '{"version":"1.0.0","podspec":"{\"name\":\"MyPod\",\"version\":\"1.0.0\"}","source":"https://github.com/you/MyPod.git"}'

curl localhost:8080/v1/packages/MyPod
curl localhost:8080/v1/search?q=MyPod
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
