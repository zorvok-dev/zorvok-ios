# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**Status: Milestone 1 (CLI skeleton).** `zorvok --version`, `--help`, `init`, `config`, and `doctor` are implemented and tested. `install`, `update`, `search`, `info`, `lib lint`, `login`, `trunk push`, and `registry *` are not — running them prints an explicit "not implemented yet" message rather than failing silently or doing nothing. No engine or registry server exists yet.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

## Try it now (Milestone 1)

```bash
go build -o zorvok ./cmd/zorvok   # Homebrew distribution lands later

./zorvok doctor     # check your local Ruby/CocoaPods/git environment
./zorvok init       # create .zorvok.yml in a project (never touches your Podfile)
./zorvok config set email you@example.com
./zorvok --help
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
