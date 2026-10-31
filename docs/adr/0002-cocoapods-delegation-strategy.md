# ADR 0002: Delegate to real CocoaPods (`pod install`) through Phase 1-2; never reimplement Xcode project generation

## Status
Proposed (Milestone 0)

## Context
CocoaPods compatibility has three distinct surfaces: format parsing (Podfile/podspec/lockfile), dependency resolution (Molinillo), and Xcode integration (`xcodeproj`). Reimplementing any of these risks silent divergence from real CocoaPods behavior, which is the single most damaging failure mode for a "drop-in compatible" tool.

## Decision
- Podfile/podspec parsing: delegate to `pod ipc podfile-json` / `pod ipc spec` (CocoaPods-Core), not reimplemented.
- `Podfile.lock` read/write: reimplemented in Go (it's a static YAML format, not a DSL), gated by golden-file tests diffing against real `pod install` output.
- Dependency resolution + Xcode project generation: delegate to the real `pod install`/`pod update` binary through Milestones 2-5 (Phase 1-2 of the brief's phased approach). Zorvok's own value in this period is entirely in *which sources* get fed to `pod install` (registry federation) and the surrounding UX (search, publish, auth) — not in replacing resolution or integration.
- Only in Milestone 6 (Phase 3+) does resolution move to a narrower Ruby shim calling `cocoapods-core` + `molinillo` + `xcodeproj` directly — still Ruby, still delegated, just bypassing the full `pod` CLI's own source-selection logic so Zorvok's federation layer becomes authoritative.
- Xcode project/workspace generation (`xcodeproj`) is **never** reimplemented, at any milestone. It encodes too much undocumented, battle-tested Xcode behavior for a rewrite to be justified by any amount of "independence from CocoaPods."

## Consequences
- Fast, low-risk path to a real `zorvok install` that works on real CocoaPods projects from Milestone 2 onward.
- Zorvok remains dependent on a working Ruby + CocoaPods install through at least Milestone 5; this is an accepted, explicit runtime dependency, not a bug.
- "Reduce/eliminate the runtime dependency on CocoaPods" (brief, Phase 4) is scoped to mean *resolution and source-of-truth*, never Xcode integration.
- Resolver-swap at Milestone 6 is the single highest-risk transition in the roadmap and must ship behind a flag with differential testing against the delegated path before becoming default (see [ARCHITECTURE.md](../../ARCHITECTURE.md) §5.2).

See [ARCHITECTURE.md](../../ARCHITECTURE.md) §2.2, §4.
