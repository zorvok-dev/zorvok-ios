# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**Status: Milestone 2, in progress (CocoaPods compatibility).** `zorvok --version`, `--help`, `init`, `config`, `doctor`, `install`, `update`, and `lib lint` are implemented and tested — `install`/`update`/`lib lint` delegate to a real local CocoaPods install (`pod install`/`pod update`/`pod lib lint`) per [ADR 0002](docs/adr/0002-cocoapods-delegation-strategy.md). A real, non-mocked compatibility test verifies this end-to-end for a local-path dependency; broader fixture coverage (git deps, subspecs, XCFrameworks, resource bundles) is still pending, see [ROADMAP.md](ROADMAP.md). `search`, `info`, `login`, `trunk push`, and `registry *` are not implemented yet — running them prints an explicit "not implemented yet" message rather than failing silently or doing nothing. No registry server exists yet.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

## Try it now (Milestone 1-2)

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
