# Zorvok Architecture

Status: **Approved and implemented.** This is the Milestone 0 design record, kept as written rather than edited into a live status page — [ROADMAP.md](ROADMAP.md) has current status, and [docs/adr/](docs/adr/) has the decisions that later corrected parts of this plan.

## 1. Problem framing

Zorvok's hard constraint is **CocoaPods compatibility**: an existing CocoaPods project must install and build with zero Podfile/podspec changes. That constraint shapes every decision below, so we start by inventorying what "CocoaPods compatibility" actually means in terms of real, load-bearing components — not a guess at what CocoaPods does, but its actual architecture.

### 1.1 How CocoaPods is actually built

CocoaPods is not one program; it's a stack of Ruby gems, each with a narrow job:

| Gem | Responsibility |
|---|---|
| `cocoapods` | The `pod` CLI itself: command routing, the `Installer`, user-facing UX. |
| `cocoapods-core` | Parses `Podfile`, `.podspec`/`.podspec.json`, and `Podfile.lock` (YAML). Owns the `Specification`, `Podfile`, and `Lockfile` object models. |
| `molinillo` | Generic backtracking dependency resolver (gem-agnostic; also used by Bundler). CocoaPods feeds it requirements extracted from the Podfile/podspecs; Molinillo returns a consistent dependency graph or proves it's unsatisfiable. |
| `cocoapods-downloader` | Fetches pod sources — git, http(s)/zip, svn, mercurial, local path. |
| `xcodeproj` | Reads/writes `.xcodeproj`/`.xcworkspace` as Ruby objects — build phases, build settings, target membership, groups. This is what turns a resolved dependency graph into an actual Xcode integration (the `Pods.xcodeproj`, the `.xcconfig` files, the workspace reference). |
| `claide` | Small CLI framework (option parsing, help text) CocoaPods and several of its plugins are built on. |

Distribution is CDN-based since CocoaPods 1.7.2: spec metadata lives at `cdn.cocoapods.org`, built via sharded index files (`all_pods.txt`, per-prefix `all_pods_versions_X_Y_Z.txt`) generated from the `CocoaPods/Specs` trunk repo and served through Fastly (originally fronted on Netlify); the old approach — cloning the ~3 GB `Specs` git repo directly — still works but is deprecated. Publishing goes through **Trunk**, a separate auth + API service (`trunk.cocoapods.org`): session tokens issued by email verification (no passwords), `pod trunk push` lints the podspec locally (including a build) before uploading, and the first pusher of a pod becomes its owner with the ability to add co-owners.

The `Podfile.lock` is a YAML document with a fixed section order (`PODS`, `DEPENDENCIES`, `SPEC REPOS`, `EXTERNAL SOURCES`, `CHECKOUT OPTIONS`, `SPEC CHECKSUMS`, `PODFILE CHECKSUM`, `COCOAPODS`) — `SPEC CHECKSUMS` pins each pod to a SHA1 of its resolved spec, and `PODFILE CHECKSUM` detects out-of-band Podfile edits. Any tool that claims to read/write this file has to reproduce that structure exactly, including key ordering, or `pod install`/`zorvok install` will treat every subsequent run as drifted.

**Strategic fact that changes the plan:** CocoaPods is now in maintenance mode (per the CocoaPods org's own public status) — not actively developed, still functional. This matters two ways: (a) there's real developer pain / opportunity here, which validates the project; (b) we can depend on CocoaPods' Ruby internals as a *stable* substrate in the short term without worrying about them shifting under us, but we also can't expect upstream fixes if we find edge-case bugs — compatibility work falls entirely on us.

### 1.2 What "compatible" has to mean operationally

Three distinct compatibility surfaces, each with a different risk profile:

1. **Format compatibility** — parse/emit `Podfile`, `.podspec`, `Podfile.lock` exactly as CocoaPods does. Low risk if we reuse CocoaPods-Core; high risk (silent divergence) if we reimplement the parser ourselves, because `Podfile`/`.podspec` are literally Ruby DSLs evaluated in a sandboxed context, not declarative config files. A from-scratch parser is a guaranteed long-tail of subtle bugs (conditionals, `pod` blocks with Ruby expressions, custom `Spec.new` logic in podspecs that call out to shell commands, etc).
2. **Resolution compatibility** — given the same inputs, produce the same resolved graph CocoaPods would. This is where Molinillo's specific backtracking behavior matters; subtle resolver differences are the single most common source of "works with pod install, breaks with the new tool" bug reports in every CocoaPods-alternative project we're aware of (this is the lesson from tools like `cocoapods-binary`-style forks and various internal forks — not something to relearn the hard way).
3. **Integration compatibility** — the generated `Pods.xcodeproj`, `.xcconfig` files, and workspace references have to match what `xcodeproj` produces closely enough that Xcode builds without complaint. `xcodeproj`'s object model encodes a lot of undocumented Xcode behavior (build phase ordering, `FRAMEWORK_SEARCH_PATHS` composition, resource bundle target wiring). Reimplementing this is the single largest, highest-risk, lowest-leverage piece of the whole project.

Given that, the architecture below treats **(1) and (3) as "do not reimplement, at least not before Milestone 6"**, and treats (2) — resolution — as the one place where early investment is both feasible and valuable, because resolution is where *our* value-add (multi-registry federation) actually lives.

## 2. Core architectural decision: thin Go CLI, CocoaPods engine as a delegate (not a dependency to replace on day one)

```
┌─────────────────────────────────────────────────────────────────┐
│ zorvok (Go binary, distributed via Homebrew)                     │
│                                                                    │
│  ┌──────────────┐   ┌──────────────┐   ┌───────────────────────┐ │
│  │ cmd/zorvok   │   │ internal/cli │   │ internal/config        │ │
│  │ (entrypoint) │→→│ (routing,    │   │ internal/auth          │ │
│  └──────────────┘   │  flags, help)│   │ (Trunk-style sessions, │ │
│                      └──────┬───────┘   │  Zorvok tokens)        │ │
│                             │            └───────────────────────┘ │
│              ┌──────────────┼──────────────────────┐              │
│              ▼              ▼                      ▼              │
│  ┌────────────────┐ ┌───────────────┐   ┌────────────────────┐   │
│  │ internal/       │ │ internal/     │   │ internal/publisher  │   │
│  │ resolver        │ │ registry      │   │ (lib lint,          │   │
│  │ (federation:    │ │ (HTTP clients:│   │  trunk push logic)  │   │
│  │  merge sources, │ │  Zorvok API,  │   └──────────┬──────────┘   │
│  │  version/owner  │ │  CocoaPods    │              │              │
│  │  conflict rules)│ │  CDN reader)  │              │              │
│  └────────┬────────┘ └───────┬───────┘              │              │
│           │                  │                       │              │
│           ▼                  ▼                       ▼              │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ engines/cocoapods/adapter                                     │  │
│  │  — the ONLY component allowed to talk to the CocoaPods        │  │
│  │    Ruby toolchain. Everything above this line is CocoaPods-   │  │
│  │    unaware.                                                   │  │
│  │                                                                 │  │
│  │  engines/cocoapods/podfile      → shells to `pod ipc podfile-  │  │
│  │                                    json` / CocoaPods-Core to   │  │
│  │                                    get a parsed, JSON Podfile  │  │
│  │  engines/cocoapods/podspec      → same, via `pod ipc spec`     │  │
│  │  engines/cocoapods/installer    → Phase 1-2: shells to real    │  │
│  │                                    `pod install` with a        │  │
│  │                                    synthesized Podfile+source  │  │
│  │                                    config; Phase 3+: calls a   │  │
│  │                                    thin Ruby "install-only"    │  │
│  │                                    shim that reuses Molinillo  │  │
│  │                                    + xcodeproj directly         │  │
│  │                                    instead of the full `pod`   │  │
│  │                                    CLI                         │  │
│  │  engines/cocoapods/compatibility → format/golden-file tests    │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                             │                                       │
│                             ▼                                       │
│              Ruby + installed CocoaPods gems (`cocoapods-core`,     │
│              `molinillo`, `cocoapods-downloader`, `xcodeproj`)      │
│              — a *runtime dependency*, not vendored/forked code.    │
│              `zorvok doctor` verifies/bootstraps this.               │
└─────────────────────────────────────────────────────────────────┘

                 (separate deployable, not in the CLI binary)
┌─────────────────────────────────────────────────────────────────┐
│ registry-server                                                   │
│  api/       — versioned REST API (packages, versions, search,     │
│               ownership, auth)                                    │
│  auth/      — publisher accounts, session/API tokens, email       │
│               verification (mirrors Trunk's UX, not its code)     │
│  packages/  — package + version metadata, ownership, dedupe/      │
│               dependency-confusion checks                         │
│  versions/  — immutable version records, deprecation flags        │
│  storage/   — PostgreSQL (metadata) + S3-compatible object store  │
│               (podspecs + source archives) + CDN in front of it   │
└─────────────────────────────────────────────────────────────────┘
```

### 2.1 Why Go for the CLI (not Ruby, not Swift)

- **Ruby** is CocoaPods' own language, and "just extend CocoaPods" was explicitly ruled out by the brief's non-goals (independent registry, independent CLI identity, eventual independence from the CocoaPods runtime). Shipping a Ruby CLI also means shipping/managing a Ruby + gem toolchain as *our* distribution problem, which is exactly the dependency-hell UX CocoaPods itself is criticized for (`rbenv`/system Ruby conflicts, native gem compilation). Not a good foundation for a Homebrew-distributed binary.
- **Swift** is attractive for "speaks the native iOS toolchain's language" but has a materially worse story for a static, dependency-free, cross-compiled CLI binary (SwiftPM-built executables are easy on macOS but the toolchain and build story for the registry server's backend — Linux, containers, CI — is worse than Go's, and the brief already separates CLI from registry-server, which will not run on macOS).
- **Go** gives: single static binary (trivial Homebrew formula, no runtime to install), fast startup (matters for a CLI invoked constantly in CI), first-class cross-compilation (arm64 + x86_64 from one toolchain, satisfying the Homebrew requirement directly), a mature HTTP/JSON stdlib for the registry client, and the same language can run the registry-server backend if we want one codebase for both (not required, but available).
- **The cost we accept:** Go does not run Ruby. The CLI cannot "just call into" CocoaPods-Core as a library the way a Ruby CLI could. It has to shell out to the `pod` executable / a small Ruby IPC shim and talk JSON over stdio/pipes. This is a real integration cost, addressed in §2.2 — it's the central engineering risk of Milestone 2, not an afterthought.

This mirrors a decision other Go-based tools that wrap Ruby/Python ecosystems have made (e.g., Go tools that shell to `bundler`/`pip` rather than reimplement dependency resolution): treat the foreign-language engine as a subprocess with a defined IPC contract, not as a library to link against.

### 2.2 Go ↔ Ruby integration: the actual mechanism

CocoaPods already ships a machine-readable interface for exactly this purpose: `pod ipc`. It exposes subcommands (`pod ipc podfile-json <Podfile>`, `pod ipc spec <podspec>`, `pod ipc list`, etc.) that print CocoaPods-Core's parsed representation as JSON to stdout. This is the integration seam:

- `engines/cocoapods/podfile` shells to `pod ipc podfile-json`, decodes the JSON into a Go struct, and that's the *only* place a Podfile's Ruby DSL gets evaluated.
- `engines/cocoapods/podspec` does the same via `pod ipc spec`.
- For `Podfile.lock`, we do **not** shell out — it's a static YAML file with a documented, stable structure (verified above). Round-tripping it in Go with a YAML library is safe and fast, *as long as we write a golden-file test suite that diffs our output against real `pod install` output byte-for-byte on representative fixtures* (ordering of `SPEC CHECKSUMS` keys, `PODFILE CHECKSUM` recomputation, etc. — see §6).
- `engines/cocoapods/installer` (Phase 1–2, Milestones 2 and 5): does not try to reimplement installation at all. It takes the resolved registry source list, synthesizes a CocoaPods-compatible `:source` configuration (pointing `pod install` at the Zorvok registry's spec CDN endpoint *and* the real CocoaPods CDN, in priority order), and shells to the real `pod install`/`pod update` binary. Zorvok's value in this phase is entirely in *what sources get fed to pod install* and *the UX around it* (`zorvok search`, `zorvok info`, `zorvok login`, `zorvok trunk push` against our registry) — not in replacing resolution or Xcode integration yet. This is deliberate under-engineering: it's the fastest path to a real, trustworthy `zorvok install` on day one.
- Only in Phase 3+ (Milestone 6) does `engines/cocoapods/installer` graduate to calling a narrower Ruby shim (a few hundred lines, not a fork of `cocoapods`) that uses `cocoapods-core` + `molinillo` + `xcodeproj` directly for tighter control — still Ruby, still a subprocess, but bypassing the full `pod` CLI's own source-resolution logic so Zorvok's federation layer is authoritative instead of CocoaPods'.

**Non-goal, explicitly:** never reimplement `xcodeproj`'s Xcode project generation in Go. That component is the highest-risk, highest-effort, lowest-differentiation part of the whole system, and CocoaPods' own implementation is the only one battle-tested against real-world Xcode quirks. It stays a Ruby dependency indefinitely, i.e. "reduce or eliminate the runtime dependency on CocoaPods" (brief, Phase 4) should be read as *resolution and source-of-truth*, not *Xcode integration*.

### 2.3 Registry federation model

The plan below was the Milestone 0 proposal and doesn't match what shipped; kept for the history, corrected in the paragraphs after it.

**The original plan:** feed `pod install` a synthesized, provenance-tagged source list spanning both registries, without touching the Podfile. This doesn't work. `pod install` has no flag to add or override a spec source for one invocation, no flag to point at an alternate Podfile, and a repo registered globally via `pod repo add` isn't implicitly searched — a Podfile with no `source` line fails against a pod only present in that repo, with CocoaPods' own error naming the problem: *"not added the source repo that hosts the Podspec to your Podfile."* A source has to be declared in the Podfile itself, because that's the only place the `pod` binary ever gets one from (ADR 0004).

**What shipped instead:** `Pod::Config#podfile` has a public writer, and `Pod::Podfile.from_ruby(path, contents)` only reads `path` from disk when `contents` is nil. Zorvok's Ruby shim (`engines/cocoapods/installer/zorvok_install.rb`, driven via `zorvok install --native`) reads the real Podfile, prepends any federated `source` lines to that content in memory, and hands the augmented string to `Pod::Installer` directly — the same class `pod install` itself calls, just given a Podfile object the file on disk never saw (ADR 0005). A pod declared nowhere in the Podfile and sourced from nowhere it names resolves correctly this way, and the Podfile's bytes are unchanged before and after. With no `--source` given, the native path's `Podfile.lock` is byte-identical to the delegated path's.

`--native` stays opt-in. The registry speaks the CocoaPods CDN protocol a `--source` URL needs (`registry-server/cdn`, ADR 0006, §2.4), and automatic federation (ADR 0008) now adds a *configured* registry without a `--source` flag at all, with ADR 0007's ambiguity refusal as the safety net underneath it.

The precedence rules below still describe the target design for when Zorvok's own federation logic picks which source(s) to pass to `--source` automatically, rather than a developer specifying one by hand:

- A pod name that is **registered as owned on the Zorvok registry** always resolves from Zorvok for that name, even if a same-named pod exists on the CocoaPods registry — this is the dependency-confusion guard (brief §9/§4 "never silently replace a package with one from a different registry").
- A pod **not present on Zorvok** falls through to CocoaPods CDN, unchanged behavior from today.
- Explicit per-pod source pinning in the Podfile (`pod 'Foo', :source => '...'` — already valid CocoaPods syntax) always wins and is never overridden by federation logic.
- Conflicting version constraints across sources for the *same* resolved pod are a hard resolution failure, surfaced with both sources named, never a silent pick.
- Provenance (registry + checksum) is recorded in an **extended** `Podfile.lock` section, additive only.

This remains intentionally conservative: not trying to out-resolve Molinillo, just feed it an unambiguous, provenance-tracked source list once Zorvok is in a position to do so at all.

### 2.4 Zorvok Registry server

**This section describes Tier 3 — `zorvok-registry`, the self-hostable server — now its own repository.** See [ADR 0009](docs/adr/0009-three-tier-registry-model.md) for the three-tier model and why: this design remains correct and unchanged for anyone running their own registry; it was never meant to be how *Zorvok's own* public registry gets hosted (that's Tier 2, `zorvok-dev/zorvok-specs` — static files, no server, see ADR 0009's "genuinely new" section on how ownership/immutability work without a database there).

Standard, boring, proven stack — intentionally no novelty here, all the novelty budget goes to compatibility and federation:

- **Metadata store:** PostgreSQL. Packages, versions, owners, download stats, deprecation flags, audit log. Relational because ownership/permissions/version-immutability are exactly the kind of invariants a relational DB with real constraints protects well (e.g., a DB-level uniqueness constraint on `(package_name, version)` is a stronger immutability guarantee than app-level checks alone).
- **Artifact store:** S3-compatible object storage (podspecs as JSON, source tarballs if we host sources rather than just pointing at git). Deliberately separate from metadata per the brief — lets us swap storage backends (S3, GCS, R2, MinIO for local dev) without touching the metadata schema.
- **CDN:** implemented (`registry-server/cdn`, ADR 0006) as a facade serving the real CocoaPods CDN source protocol, read directly from the `cocoapods-core` gem source rather than guessed, in front of `registry-server/packages`. Mounted at `/cdn/`, this is what lets `zorvok install --native --source <registry-url>/cdn/` work against a real Zorvok registry. It skips `cdn.cocoapods.org`'s MD5-prefix sharding (`prefix_lengths: []`) — that exists for a registry with hundreds of thousands of pods, which Zorvok is nowhere near. Responses are computed on the fly from the in-memory store rather than from precomputed index files; worth revisiting once there's a persistent backend and real traffic, same path `cdn.cocoapods.org` itself took.
- **API:** versioned REST (`/v1/...`) per the brief. Auth mirrors Trunk's UX (email-verified, per-machine session/API tokens) rather than requiring upfront password accounts — lowers the barrier to publish, which is the entire point of a healthy registry.
- **Explicitly out of scope for the server:** executing any code from a published package. Podspecs are declarative metadata; "zorvok lib lint" validation runs *client-side* (on the publisher's machine, via the CocoaPods linting machinery already available there) and the server re-validates structurally (schema, checksum, duplicate-version, ownership) without ever building or running the package's code (brief §9: "never execute arbitrary podspec or package scripts on the registry server").

## 3. Repository structure

**Superseded by [ADR 0009](docs/adr/0009-three-tier-registry-model.md).** The single-monorepo layout originally planned here (including `registry-server/` and `website/`) was the Milestone 0 plan; it held through Milestone 8, then split into three repositories once the registry itself split into tiers. The structure below is what actually exists, not the original plan:

**`zorvok-dev/zorvok-ios`** (this repo — the CLI):
```
zorvok/
├── cmd/zorvok/                   # main package, flag parsing entry point only
├── internal/
│   ├── cli/                      # command routing (install, update, search, login, ...)
│   ├── config/                   # ~/.zorvok config, per-project .zorvok.yml
│   ├── auth/                     # local credential storage (per-registry-host session tokens)
│   ├── registry/                 # Go HTTP client for the registry REST API
│   ├── registrytest/             # starts a real zorvok-registry subprocess for tests (ADR 0009)
│   ├── installer/                # orchestrates engines/cocoapods
│   └── publisher/                # lib lint + trunk push orchestration
├── engines/cocoapods/
│   ├── adapter/                  # subprocess/IPC boundary — the only Ruby-calling code
│   ├── podspec/                  # podspec → JSON (via `pod ipc spec`)
│   ├── installer/                # Phase 1-2: shells to `pod install`; Phase 3: Ruby shim (ADR 0005)
│   └── compatibility/            # fixture-diff tests against real CocoaPods
├── docs/adr/                      # architecture decision records, including this repo split
├── tests/fixtures/                # real-world Podfiles/podspecs for compatibility + federation testing
├── ARCHITECTURE.md, ROADMAP.md
```

**[`zorvok-dev/zorvok-registry-ios`](https://github.com/zorvok-dev/zorvok-registry-ios)** (Tier 3, self-hostable):
```
zorvok-registry/
├── cmd/zorvok-registry/
├── api/ auth/ cdn/ packages/ storage/
```

**[`zorvok-dev/zorvok-specs`](https://github.com/zorvok-dev/zorvok-specs)** (Tier 2, Zorvok's own free public registry):
```
specs/
├── CocoaPods-version.yml, all_pods.txt, all_pods_versions.txt   # regenerated, not hand-edited
├── Specs/<name>/OWNERS, <version>/<name>.podspec.json            # published content
├── scripts/validate-pr.sh, generate-index.sh                     # the ownership/immutability enforcement a server's database would otherwise provide
└── .github/workflows/                                            # wires the scripts above to PRs/merges
```

`website/` and `homebrew/` (Milestone 7, public launch) haven't been started.

## 4. CocoaPods compatibility assessment

| Surface | Strategy | Risk | Milestone |
|---|---|---|---|
| Podfile parsing | Delegate to `pod ipc podfile-json` | Low (CocoaPods' own parser) | M2 |
| Podspec parsing | Delegate to `pod ipc spec` | Low | M2 |
| Podfile.lock read/write | Reimplement in Go (static YAML format) | **Medium** — must byte-match `pod install` output; needs golden-file tests from day one | M2 |
| Dependency resolution | Delegate (Phase 1-2: via `pod install`; Phase 3+: Ruby shim calling Molinillo directly) | Low while delegating; Medium once we take it over | M2 → M6 |
| Subspecs, git/local/path deps, binary frameworks, XCFrameworks, resource bundles | Inherited for free while delegating to `pod install` | Low (M2-M5), re-tested continuously | M2 |
| Xcode project/workspace generation | **Never reimplement** — stays on `xcodeproj` indefinitely | High if ever attempted; avoided by design | n/a |
| Multiple targets/platforms, build configs | Inherited for free while delegating | Low | M2 |
| CocoaPods plugins | Deferred — plugins hook into the `pod` CLI's Ruby plugin API; only reachable while we still shell to real `pod install` (Phase 1-2). Revisit explicitly before Phase 3 removes that path, or document the loss. | Medium (scope not yet sized) | Post-M2, revisit before M6 |

**We will not claim "CocoaPods compatible" publicly until the acceptance test in §6 passes on a real, unmodified existing CocoaPods project**, per the brief's own instruction. As of Milestone 2's current state: `zorvok install`/`update`/`lib lint` work and are verified end-to-end against real (not mocked) CocoaPods for simple, transitive, subspec, git, resource-bundle, multi-target, and XCFramework fixtures — see the coverage table in [ROADMAP.md](ROADMAP.md) Milestone 2. Only an existing hand-authored `Podfile.lock` from a different CocoaPods version/project is not yet covered, so the broader compatibility claim is not yet made.

## 5. Technical risks

1. **Podfile.lock byte-fidelity** — our biggest near-term risk. If Zorvok's lockfile writer diverges from `pod install`'s in formatting, key order, or checksum algorithm, a team with mixed `pod`/`zorvok` usage gets spurious diffs and false "drifted" states. Mitigation: golden-file tests against real `pod install` output pinned to specific CocoaPods versions, run in CI, before this ships.
2. **Resolver semantics drift** once we move off delegated `pod install` (Phase 3+, M6) — different resolution results for the same Podfile depending on whether a user ran `pod install` or `zorvok install` is the worst possible failure mode (silent, hard to debug, erodes trust). Mitigation: Phase 3 ships behind a flag with a large differential test corpus (resolve N real-world Podfiles with both engines, diff the result) before becoming default.
3. **`pod` / Ruby toolchain availability.** Delegating to `pod install` means Zorvok still requires a working CocoaPods Ruby install during Phases 1-2. `zorvok doctor` must detect, diagnose, and offer to bootstrap this (consistent with the brief's `zorvok doctor` command) — this is a real UX risk if not handled well, since "install Ruby correctly" is itself a classic source of CocoaPods pain.
4. **Dependency confusion / name collision across registries.** Real CocoaPods resolves two sources offering the same pod/version by declaration order, with only a warning — not a hard failure. Reproduced directly against real CocoaPods before fixing it (ADR 0007): `zorvok install --native` now refuses, before any download, to resolve a pod that's ambiguous across sources with no explicit `:source =>` pin, whether the extra source came from `--source` or from automatic federation (ADR 0008). The collision-*free* case — an unclaimed or plausible-sounding name resolving silently from an open registry with nothing to collide against — is real but isn't novel to Zorvok: the public CocoaPods CDN carries the identical exposure today, and the mitigation lives at a registry's own ownership/moderation layer, not in installer-side resolution. §2.3's original "a Zorvok-owned name always wins ties" rule is superseded by this hard-refuse policy, which is the permanent behavior, not an interim one.
5. **CDN/infra cost and ownership** for hosting source archives, not just metadata — brief says "evaluate," not "commit to hosting." Initial registry design should support **source-by-reference** (podspec points at a git tag/tarball URL, same as CocoaPods today) as the default, with Zorvok-hosted artifact storage as an *opt-in* enhancement, to avoid an unbounded storage/bandwidth cost commitment in the MVP.
6. **CocoaPods maintenance-mode status** is double-edged: stable target to integrate against short-term, but means Zorvok inherits sole responsibility for any CocoaPods bugs it surfaces along the way (no upstream to escalate to).

## 6. MVP / acceptance definition

The brief's own acceptance test is the right one and should gate the "CocoaPods compatible" claim:

```
cd ExistingiOSProject   # real, unmodified, pre-existing CocoaPods project
zorvok install
# → installs successfully, produces a working Xcode project,
#   project builds in Xcode, with zero Podfile/podspec changes
```

Concretely, M2 ("CocoaPods compatibility") is not done until:
- `tests/fixtures/` contains real sample projects covering: simple single-dependency, multiple dependencies, transitive deps, subspecs, git deps, local path deps, XCFramework binary, resource bundles, multi-target, and an existing hand-written `Podfile.lock`.
- `tests/compatibility/` runs both `pod install` and `zorvok install` against each fixture and diffs: resulting `Podfile.lock`, resulting `Pods.xcodeproj` build settings (via `xcodeproj` introspection, not raw diff — the file format isn't meant to be byte-stable across unrelated runs), and an actual `xcodebuild` of the generated project.
- All of the above are green in CI before `zorvok install` is advertised as CocoaPods-compatible anywhere in docs/marketing.

## 7. Open questions for approval

1. Registry source-hosting model (§5.5): source-by-reference (git/tarball URL, like CocoaPods) vs. Zorvok-hosted artifacts — recommend starting reference-only for MVP; confirm before M3.
2. Scope of CocoaPods plugin support (§4) — recommend explicitly deferring and documenting as a known gap for M2-M5, revisited before M6; confirm.
3. Whether `zorvok doctor` should be able to *install* a working Ruby/CocoaPods toolchain for the user, or only diagnose and point at instructions — affects M1 scope.

Pending approval of this document, the proposed next step is **Milestone 1**: `zorvok --version`, `--help`, `doctor`, `config`, `init` — no CocoaPods interaction yet, establishing the CLI skeleton and config/auth foundations in `internal/`.
