# ADR 0004: Automatic registry federation is not possible while delegating to `pod install`; defer to Milestone 6

## Status
Accepted (supersedes the federation mechanism assumed in ARCHITECTURE.md §2.3 and ADR 0002/0003)

## Context
ARCHITECTURE.md §2.3 assumed Milestone 5 could federate `zorvok install` across the Zorvok and CocoaPods registries by having Zorvok "synthesize a CocoaPods-compatible `:source` configuration" and feed it to the real `pod install` binary (Phase 1-2 delegation, per ADR 0002) — without modifying the user's Podfile, consistent with Zorvok's core promise and with the compatibility test suite's own enforced invariant (`TestInitDetectsPodfileWithoutModifyingIt` et al.).

While starting Milestone 5, this was tested empirically rather than assumed:

1. `pod install --help` has no flag to add or override spec sources for a single invocation (`--repo-update`, `--deployment`, `--clean-install`, `--project-directory`, `--allow-root`, `--silent`, `--verbose`, `--no-ansi` — nothing else).
2. A Specs-formatted repo registered globally via `pod repo add` is **not** implicitly searched. A Podfile with no `source` line, run against a pod only present in that added repo, fails with: *"not added the source repo that hosts the Podspec to your Podfile."* CocoaPods repos are opt-in per-Podfile by design (itself a reasonable anti-dependency-confusion choice on CocoaPods' part).
3. There is no `pod install` flag to point at an alternate Podfile path; it always reads the literal file named `Podfile` in the working/project directory.

**Conclusion: a source must be declared inside the Podfile itself for `pod install` to search it. There is no mechanism to add a source "invisibly" for one run.**

## Decision
Automatic, zero-Podfile-touch registry federation is **not implemented in Milestone 5**. It is deferred to **Milestone 6** (ADR 0002 Phase 3+), where Zorvok's own Ruby shim calls `cocoapods-core` + `molinillo` directly instead of going through the full `pod` CLI's own source-resolution logic — at that point Zorvok fully controls the source list programmatically and no Podfile declaration is needed.

Until Milestone 6, using the Zorvok registry works exactly the way adding any other private CocoaPods registry already works today: the user adds one `source 'https://...'` line to their own Podfile, the same step they'd take to use any CocoaPods private spec repo. This is explicitly **not** a Zorvok-specific migration step — it's normal, pre-existing CocoaPods practice, and it does not conflict with the "no Podfile changes to keep using your existing CocoaPods dependencies" promise, which is about not having to rewrite *existing* dependency declarations, not about Zorvok packages being discoverable with literally no Podfile awareness of them.

## Rejected alternative: transient Podfile edit-then-revert
Append the source line, run `pod install`, revert the file immediately after. Rejected: even though the file would be restored byte-for-byte, the Podfile is genuinely modified on disk for the duration of the run — this directly contradicts the invariant the compatibility suite enforces and that Zorvok's entire pitch rests on, for no benefit a user-added `source` line doesn't already provide more honestly. Revisit only if Milestone 6 is abandoned and Phase 1-2 delegation must remain permanent (not the current plan).

## Consequences
- **Milestone 5 (this repo's roadmap) is re-scoped**: instead of implicit federation, it becomes "document and support opt-in use of the Zorvok registry via a user-added `source` line," plus whatever dependency-confusion-safe tooling helps with that (e.g. `zorvok registry add` as a thin wrapper around `pod repo add` with Zorvok-specific guidance, and a `zorvok doctor` check for a misconfigured/missing source). See ROADMAP.md.
- ARCHITECTURE.md §2.3 and §5 (risks) need correction to stop describing a mechanism that doesn't exist; done in the same change as this ADR.
- The dependency-confusion precedence rules in ADR 0003 §Federation precedence still apply once a source line is present — they describe what Zorvok's *own* resolver should do once it owns resolution (Milestone 6), and what documentation should tell users about explicit per-pod source pinning in the meantime.
- This is a concrete example of the exact failure mode ARCHITECTURE.md §1.1 already warned about: "Do not assume that querying two registries automatically gives a correct combined dependency graph" — investigating CocoaPods' actual behavior here, rather than assuming based on the written plan, caught a real gap before any code was built on top of a false premise.
