# ADR 0006: `registry-server/cdn` — serving the Zorvok registry as a CocoaPods CDN source

## Status
Accepted (Milestone 6, second increment)

## Context
ADR 0005's native installer can inject a Podfile `source` URL in memory and resolve a pod from it — proven against a hand-built local git Specs repo. For `zorvok install --native --source <url>` to work against the *actual* Zorvok registry (`cmd/zorvok-registry`), that server has to respond at `<url>` the way CocoaPods expects a spec source to respond. ARCHITECTURE.md §2.4 already named the target shape ("same shape as `cdn.cocoapods.org`'s sharded-index approach") but the exact protocol was not implemented against — it needed to be read from the actual `cocoapods`/`cocoapods-core` gem source, not assumed, the same discipline ADR 0004/0005 used.

## Decision
Read directly from the installed gems (`Pod::Source::Manager#cdn_url?`, `Pod::CDNSource`, `Pod::Source::Metadata#path_fragment`) before writing any server code. The protocol, confirmed from that source:

1. **CDN detection**: the first time CocoaPods sees a fresh `source 'https://...'` URL, `cdn_url?` fetches `<url>/CocoaPods-version.yml` and treats the source as a CDN (not something to `git clone`) if that file parses as YAML with a non-nil `last` key. On success, CocoaPods does the local-side equivalent of `pod repo add-cdn` (writes a `.url` marker file) and never clones anything.
2. **Sharding**: `Source::Metadata#path_fragment` shards pod names via `prefix_lengths` (an MD5-prefix scheme) — but if `prefix_lengths` is empty, there is no sharding at all: every pod's spec lives directly under `Specs/<name>/`, and the single version-index file is named `all_pods_versions.txt` with no shard suffix. Zorvok's registry declares `prefix_lengths: []` — real `cdn.cocoapods.org`'s 3-level MD5 sharding exists to keep any one directory from holding hundreds of thousands of entries; Zorvok's registry is nowhere near that scale, so the extra complexity buys nothing yet (revisit if/when it does).
3. **Required endpoints**, all read-only, computed from `registry-server/packages.Store` on request (no precomputed static files yet — see Consequences):
   - `GET /CocoaPods-version.yml` — `last: '1.16.2'`, `prefix_lengths: []`.
   - `GET /all_pods.txt` — one pod name per line.
   - `GET /all_pods_versions.txt` — one line per pod, `<name>/<v1>/<v2>/...` (CDNSource re-sorts versions itself; file order doesn't matter).
   - `GET /Specs/<name>/<version>/<name>.podspec.json` — the podspec, served verbatim from `packages.Version.Podspec`, same "never execute it" rule as the rest of this registry (brief §9). A pod not on this registry 404s here — which is exactly the fallthrough behavior ADR 0003's federation precedence needs when a Podfile declares multiple sources.

Mounted at `/cdn/` on the existing `registry-server/api.Server` (`http.StripPrefix("/cdn", cdn.Handler(...))`), so a Podfile `source` line (or `zorvok install --native --source`) points at `<registry base URL>/cdn/`.

## Verified before claiming it works
- Unit/httptest coverage for each endpoint's exact format (`registry-server/cdn/cdn_test.go`), including the two negative cases that matter most: a filename that doesn't match CDNSource's own request shape, and a pod not on this registry — both must 404 cleanly, not error.
- A full real-stack proof, not just the Go test suite: started a real `cmd/zorvok-registry`, published a real podspec to it with the real `zorvok trunk push` (pointed at a real, separately-hosted local git repo as the pod's actual source, since a full `install` — unlike `lib lint` — really does fetch `:source`), then ran the compiled `zorvok` binary's `install --native --source http://localhost:8099/cdn/` against a Podfile that names the pod only by name and version, nothing else. CocoaPods' own log confirmed it recognized the registry as a CDN (`Adding spec repo ... with CDN http://localhost:8099/cdn/`), resolved and installed the pod, and `Podfile.lock` recorded `SPEC REPOS: http://localhost:8099/cdn/: - CDNFedPod` — provenance tracking (ADR 0003) for free, not something Zorvok had to implement itself. The Podfile on disk was confirmed byte-unchanged.

## Consequences
- **This closes the mechanical gap ROADMAP.md flagged after ADR 0005**: `--native --source` now works against a real Zorvok registry, not only hand-built fixtures.
- **Still opt-in only** (`--native`), unchanged from ADR 0005 — `zorvok install` without `--native` never touches this endpoint.
- **On-the-fly computation, not precomputed static files.** ARCHITECTURE.md §2.4 recommended generating sharded index files on publish so reads degrade to static file serving (the single biggest lesson from `cdn.cocoapods.org`'s own migration — reads vastly outnumber writes). This increment computes `all_pods.txt`/`all_pods_versions.txt` from the in-memory store on every request instead. Fine at today's scale (one in-memory store, no persistence yet per ROADMAP.md Milestone 3); revisit once there's a PostgreSQL backend and real traffic to make static pregeneration worth the complexity.
- **Still no automatic federation.** This makes the Zorvok registry *usable* as a `--source`, same as ADR 0005 made federation technically possible — a developer (or, later, Zorvok's own tooling) still has to supply the URL explicitly. ADR 0003's precedence/ownership rules for *automatically* picking which source(s) to search, and the dependency-confusion adversarial review that must gate any automatic (non-explicit) federation, remain future work.
- **Not yet handled**: `pod repo update` against a Zorvok CDN source, deprecated-pod signaling (`deprecated_podspecs.txt` — `CDNSource#deprecated_local_podspecs`), and `prefix_lengths` sharding if/when registry scale needs it. None of these block the `--native --source` path working correctly today; they're listed so they're not silently assumed to work.
