# Zorvok

Zorvok is a CocoaPods-compatible iOS dependency manager and independent public package registry. The goal: an existing CocoaPods project should be able to run `zorvok install` instead of `pod install` — no Podfile changes, no podspec changes, no migration to Swift Package Manager — while gaining access to an independently publishable Zorvok registry alongside the existing CocoaPods ecosystem.

**Status: Milestone 0 (architecture).** Nothing below is implemented yet. This repository currently contains only the architecture proposal and directory skeleton; no CLI, no engine, no registry server exists yet.

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, technology decisions, CocoaPods compatibility assessment, registry design, risks, MVP acceptance criteria
- [docs/adr/](docs/adr/) — individual architecture decision records
- [ROADMAP.md](ROADMAP.md) — milestone plan and current status

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
