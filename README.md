# Zorvok

Zorvok is a CocoaPods-compatible dependency manager and package registry client for iOS. The goal is stated precisely because it's easy to get wrong: an existing CocoaPods project should be able to run `zorvok install` in place of `pod install` — **no Podfile changes, no podspec changes, no migration to Swift Package Manager** — and, from there, optionally gain access to one or more independent package registries alongside the CocoaPods ecosystem it already depends on. Zorvok is meant to be *an option for* CocoaPods, not a replacement that forces a migration.

This repository is the `zorvok` CLI. It is iOS-only — no Android, Maven, or Gradle functionality is in scope.

## How it works, in one paragraph

Zorvok does not reimplement CocoaPods. Dependency resolution (Molinillo), Podfile.lock generation, and Xcode project integration (`xcodeproj`) are all exactly CocoaPods' own code — by default, `zorvok install`/`zorvok update`/`zorvok lib lint` simply shell out to the real `pod` binary you already have installed (`gem install cocoapods`), so Zorvok can never diverge from CocoaPods' actual installed behavior. Everything registry-related — `trunk push`, `search`, `info`, multi-source federation — is Zorvok's own addition layered on top. The one deeper piece Zorvok does own is `zorvok install --native`, an opt-in installer that drives CocoaPods' own `Pod::Installer` class directly (not a reimplementation of it) so it can resolve a dependency from an additional registry that is declared nowhere in your Podfile, without ever writing to that Podfile. See [ARCHITECTURE.md](ARCHITECTURE.md) and [docs/adr/](docs/adr/) for the full reasoning behind each of these decisions.

## The registry ecosystem: three tiers, one protocol

A Zorvok-aware (or even a plain CocoaPods) project can resolve dependencies from any combination of these, simultaneously, with no special-casing per source:

| Tier | What | Repository | Cost to run |
|---|---|---|---|
| 1 | CocoaPods' own public registry (`cdn.cocoapods.org`) | — (unchanged, external) | Free, already there |
| 2 | Zorvok's own free public registry | [zorvok-dev/zorvok-specs](https://github.com/zorvok-dev/zorvok-specs) — static files in git, served via GitHub Pages/raw git, no server | Free |
| 3 | A self-hosted registry, for private/enterprise use | [zorvok-dev/zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios) — a real Go server, Nexus/Artifactory-style | Whatever you host it on |

All three speak the same CocoaPods CDN source protocol, so they're reached the exact same way: `zorvok install --native --source <url>`, or automatically once `registry_url` is configured (`zorvok config set registry_url <url> --global` — see [ADR 0008](docs/adr/0008-automatic-federation.md)). `zorvok install` doesn't know or care which kind of thing answers at that URL. (Rationale for the three-tier split: [ADR 0009](docs/adr/0009-three-tier-registry-model.md).)

## Installing the CLI

**Build from source** (the verified, documented path):

```bash
git clone https://github.com/zorvok-dev/zorvok-ios.git
cd zorvok-ios
go build -o zorvok ./cmd/zorvok
./zorvok --version
```

**`go install`, no clone needed** (works today against this public repository — verified against the real module path):

```bash
go install github.com/zorvok-dev/zorvok-ios/cmd/zorvok@main
```

**Homebrew — not live yet.** A real, verified Homebrew formula exists at [zorvok-dev/homebrew-tap](https://github.com/zorvok-dev/homebrew-tap) (`brew install`/`brew test`/`brew audit --strict`/`brew style` all pass against it), but `brew install zorvok` can't work until this repository has a tagged release for the formula's `url`/`sha256` to resolve against — see that tap repo's own README for the exact release process. Once that lands, the install step will be exactly:

```bash
brew install zorvok-dev/tap/zorvok
```

Requires a Go toolchain matching [go.mod](go.mod)'s `go` directive to build, and a working `gem install cocoapods` (real CocoaPods) on `PATH` for anything beyond `--version`/`--help`/`init`/`config` — run `zorvok doctor` after installing to confirm your environment.

## Getting started with an existing CocoaPods project

```bash
zorvok doctor                  # checks Ruby/CocoaPods/git are present and working
cd /path/to/your/ios/project    # the one with your existing Podfile
zorvok init                     # creates .zorvok.yml — never touches Podfile/Podfile.lock/*.podspec
zorvok install                  # delegates straight to `pod install` — byte-identical behavior
zorvok update                   # delegates straight to `pod update`
```

At this point nothing about your project has changed from a plain CocoaPods workflow — `zorvok install`/`zorvok update` are drop-in replacements, verified byte-for-byte identical to the delegated `pod install`/`pod update` they wrap (see [ARCHITECTURE.md](ARCHITECTURE.md) and the compatibility fixtures under `tests/fixtures/`).

### Opting in to Zorvok's own registries

To resolve a dependency from Zorvok's free public registry (Tier 2) or your own self-hosted `zorvok-registry-ios` server (Tier 3), without declaring a `source` line in your Podfile:

```bash
# One-off, for a single install:
zorvok install --native --source https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/

# Or configure a registry once and have it federate automatically from then on:
zorvok config set registry_url https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/ --global
zorvok install --native
zorvok install --native --no-registry   # opt out for a single run, once configured
```

`--native` is required to use `--source`/auto-federation: `pod install` itself has no mechanism to use an extra source not declared in the Podfile (confirmed against real CocoaPods, not assumed — see [ADR 0004](docs/adr/0004-federation-requires-podfile-source-line.md)). `--native` drives CocoaPods' own `Pod::Installer` directly instead, verified to produce a byte-identical `Podfile.lock` to the delegated path when no extra source is used, and to never modify the Podfile on disk either way ([ADR 0005](docs/adr/0005-native-installer-and-federation-mechanism.md)). A same-name, same-version pod resolvable from two different sources is hard-refused before anything is downloaded, rather than silently picked by source declaration order — a real CocoaPods behavior reproduced and fixed before this was automated ([ADR 0007](docs/adr/0007-federation-safety-fixes.md)).

### Publishing your own package

```bash
zorvok login you@example.com "Your Name"
# a verification link is printed to the registry server's own log (no real email
# transport is implemented yet — see Known gaps below) — open it before publishing

cd /path/to/your/pod/with/a/single/MyPod.podspec
zorvok trunk push                      # lints, converts via `pod ipc spec`, publishes
zorvok trunk info MyPod                # confirm it published
zorvok trunk add-owner MyPod teammate@example.com
zorvok trunk deprecate MyPod --in-favor-of=BetterPod
```

`trunk push` publishes to whichever registry `registry_url` is configured to (defaults to `http://localhost:8080`, a local dev server — never a hardcoded production endpoint). It does not yet publish directly to the static `zorvok-specs` registry (Tier 2) — that one is published to via pull request; see [its README](https://github.com/zorvok-dev/zorvok-specs#publishing-a-package).

## Command reference

Every command below is implemented today. Anything that delegates to `pod` requires a real CocoaPods install (`gem install cocoapods`) on `PATH`; `zorvok doctor` checks this for you.

**General**

| Command | What it does |
|---|---|
| `zorvok --version` / `zorvok version` | Print the Zorvok version and build info |
| `zorvok --help` / `zorvok help` | Show command help |
| `zorvok init` | Create a project-local `.zorvok.yml`; never touches Podfile/Podfile.lock/podspecs |
| `zorvok config [get KEY\|set KEY VALUE\|unset KEY] [--global]` | With no subcommand, lists effective settings. Project-local by default (like `git config`); `--global` targets `~/.config/zorvok/config.yml` |
| `zorvok doctor` | Diagnose the local Ruby/CocoaPods/git/Zorvok environment |

**Dependency management** — the core workflow, delegating to real CocoaPods by default

| Command | What it does |
|---|---|
| `zorvok install [--native [--source <url>...] [--no-registry]]` | Install dependencies from the Podfile. Plain form delegates to `pod install`; `--native` opts in to Zorvok's own installer (see above) |
| `zorvok update [pod-names...]` | Update outdated dependencies and refresh `Podfile.lock` (delegates to `pod update`) |
| `zorvok outdated` | Show outdated dependencies in the current `Podfile.lock` (delegates to `pod outdated`) |
| `zorvok list` | List all available pods (delegates to `pod list`) |
| `zorvok try <name\|URL>` | Download a pod's demo project and open it (delegates to `pod try`) |
| `zorvok env` | Display the local CocoaPods/Ruby/Xcode environment (delegates to `pod env`) |
| `zorvok setup` | Set up the local CocoaPods environment (delegates to `pod setup`) |
| `zorvok deintegrate [XCODE_PROJECT]` | Remove all traces of CocoaPods from an Xcode project (delegates to `pod deintegrate`) |

**Developing a pod**

| Command | What it does |
|---|---|
| `zorvok lib create NAME [--template-url=URL]` | Scaffold a new pod (delegates to `pod lib create`) |
| `zorvok lib lint [podspec]` | Validate a podspec by building it (delegates to `pod lib lint`) |
| `zorvok spec cat\|create\|edit\|lint\|which ...` | Manage podspecs (delegates to `pod spec`) |
| `zorvok ipc podfile\|podfile-json\|spec\|list\|repl\|update-search-index ...` | Convert a Podfile/podspec to YAML/JSON, or other inter-process helpers (delegates to `pod ipc`) |
| `zorvok cache clean\|list ...` | Inspect or clear the local pod download cache (delegates to `pod cache`) |
| `zorvok plugins [create\|installed\|list\|publish\|search] ...` | List, search, or scaffold CocoaPods plugins (delegates to `pod plugins`) |

**Registries — spec sources** (wraps `pod repo`, i.e. CocoaPods' own spec-repo management — works with any CocoaPods-compatible source, Zorvok or not)

| Command | What it does |
|---|---|
| `zorvok registry list` | List registered spec sources |
| `zorvok registry add NAME URL` | Register a git-backed spec source |
| `zorvok registry add-cdn NAME URL` | Register a CDN-backed spec source (what a Zorvok registry speaks) |
| `zorvok registry remove NAME` | Unregister a spec source |
| `zorvok registry update [NAME]` | Update one or all registered spec sources |
| `zorvok registry lint NAME` | Validate every spec in a repo |
| `zorvok registry push NAME [PODSPEC]` | Push specs to a git-backed spec repo |

**Publishing and accounts — `trunk`** (Zorvok's own registry API — see `internal/registry` and [zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios))

| Command | What it does |
|---|---|
| `zorvok login EMAIL [NAME]` / `zorvok trunk register EMAIL [NAME]` | Register (or re-register, to resend verification) an account and store a session token for this machine |
| `zorvok trunk push [PODSPEC] [--allow-warnings] [--skip-lint]` | Lint, convert, and publish a podspec to the configured registry |
| `zorvok trunk me` | Show the account behind this machine's stored session |
| `zorvok trunk add-owner POD OWNER-EMAIL` | Grant another account publish rights on a pod you own |
| `zorvok trunk remove-owner POD OWNER-EMAIL` | Revoke an account's publish rights (refuses to remove a pod's last owner) |
| `zorvok trunk deprecate NAME [--in-favor-of=OTHER]` | Mark a pod as deprecated, optionally pointing at a replacement |
| `zorvok trunk delete NAME VERSION` | Does **not** delete — explains why (published versions are immutable) and points at `trunk deprecate` instead |
| `zorvok search [query]` / `zorvok trunk info NAME` · `zorvok info NAME` | Search the configured registry, or show one package's details |

Run `zorvok <command> --help` (for anything that delegates to `pod`) or `zorvok help` for the full list with one-line summaries.

## Architecture and design decisions

- [ARCHITECTURE.md](ARCHITECTURE.md) — component design, CocoaPods compatibility assessment, registry design, risks, acceptance criteria
- [docs/adr/](docs/adr/) — one architecture decision record per major decision, including the dead ends (e.g. [ADR 0004](docs/adr/0004-federation-requires-podfile-source-line.md), where testing the real mechanism before building on it caught that the original plan wasn't achievable, and the re-scope that followed)
- [ROADMAP.md](ROADMAP.md) — milestone history and current status

## Running the test suite

```bash
go test ./...
```

Most packages are self-contained. A few need extra local setup to exercise real integrations rather than mocks, and skip (not fail) without it:

- `engines/cocoapods/*` and `internal/publisher` need a real `pod`/`git` on `PATH` — the actual compatibility guarantee this project makes is only meaningful tested against the real tool.
- `internal/registry`/`internal/publisher`'s registry integration tests run a real `zorvok-registry` binary as a subprocess (`internal/registrytest`), since that package now lives in a separate repository ([ADR 0009](docs/adr/0009-three-tier-registry-model.md)). Build it first and put it on `PATH`:
  ```bash
  git clone https://github.com/zorvok-dev/zorvok-registry-ios.git ../zorvok-registry-ios
  cd ../zorvok-registry-ios && go build -o /usr/local/bin/zorvok-registry ./cmd/zorvok-registry
  ```

If your terminal isn't using a UTF-8 locale, set `LANG=en_US.UTF-8` first — CocoaPods itself requires it and will otherwise emit confusing Ruby encoding errors from scripts it runs, not a Zorvok bug.

## Known gaps

Stated plainly rather than silently assumed away:

- `zorvok-registry-ios` has no PostgreSQL backend yet (in-memory store only — data is lost on restart) and no real email transport (verification links are logged, not emailed).
- `zorvok trunk push` doesn't yet publish to the static `zorvok-specs` registry (Tier 2) — only to a live `zorvok-registry-ios` server. Publishing to `zorvok-specs` is PR-based for now; see [its README](https://github.com/zorvok-dev/zorvok-specs#publishing-a-package).
- No reserved-name or publisher-reputation policy beyond "verified email, first publisher wins" (registry) / GitHub-PR ownership (static registry).
- `brew install zorvok` isn't live yet — see [Installing the CLI](#installing-the-cli) above.

See [ROADMAP.md](ROADMAP.md) for the full status, milestone by milestone.

## Related repositories

- [zorvok-dev/zorvok-specs](https://github.com/zorvok-dev/zorvok-specs) — the free public registry (Tier 2)
- [zorvok-dev/zorvok-registry-ios](https://github.com/zorvok-dev/zorvok-registry-ios) — the self-hostable registry server (Tier 3)
- [zorvok-dev/homebrew-tap](https://github.com/zorvok-dev/homebrew-tap) — the Homebrew formula (not yet publishable — see above)

## Scope

Zorvok is iOS-only. No Android, Maven, or Gradle functionality is in scope.

## Contributing

Not yet open for external contribution — there is no `CONTRIBUTING.md` yet. Issues and discussion are welcome in the meantime.

## License

MIT — see [LICENSE](LICENSE).
