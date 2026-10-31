# ADR 0003: Registry federation precedence rules, and storage stack

## Status
Proposed (Milestone 0)

## Context
Zorvok must resolve dependencies from both its own registry and the official CocoaPods registry (and optionally private registries) without silent package-name collisions, and needs a registry server design for its own public registry.

## Decision

### Federation precedence (`internal/resolver`)
1. Explicit per-pod source pins in the Podfile always win, unconditionally.
2. A pod name explicitly owned/registered on the Zorvok registry always resolves from Zorvok for that name, even if a same-named CocoaPods pod exists — this is the dependency-confusion guard.
3. A pod not present on Zorvok falls through to the CocoaPods CDN, unchanged from today's behavior.
4. Conflicting version constraints for the same pod across sources is a hard resolution failure, reported with both sources named — never a silent pick.
5. Provenance (source + checksum) for every resolved pod is recorded in an additive `Podfile.lock` extension, not by mutating the standard CocoaPods lockfile sections.

### Registry server storage
- Metadata: PostgreSQL (relational constraints enforce version immutability and ownership uniqueness at the DB level, not just in application code).
- Artifacts: S3-compatible object storage, separate from metadata.
- Distribution: CDN in front of storage, using a sharded static-index approach modeled on `cdn.cocoapods.org` (generate index files on publish; reads hit static files, not the API, for the common case).
- Source hosting model for MVP: **reference-only** (podspec points at a git tag/tarball URL, exactly like CocoaPods today), with Zorvok-hosted artifact storage as an opt-in later enhancement — avoids an open-ended storage/bandwidth cost commitment before there's registry traffic to justify it.

## Alternatives considered
- Simple two-registry concatenation ("ask both, take whichever answers first/last"): rejected explicitly — this is exactly the dependency-confusion vector the brief calls out, and produces non-reproducible results depending on network timing.
- Zorvok-hosted artifacts as the default from day one: rejected for MVP on cost/scope grounds; revisit once registry adoption data exists.

## Consequences
- Needs an adversarial security review of the ownership-precedence rule before Milestone 5 ships (can an attacker register a Zorvok package under a popular existing CocoaPods pod's name to hijack resolution?).
- Lockfile extension must be strictly additive — a project managed by Zorvok must still be readable (if not fully reproducible) by a plain `pod install` if a developer runs it directly.

See [ARCHITECTURE.md](../../ARCHITECTURE.md) §2.3, §2.4, §5.4.
