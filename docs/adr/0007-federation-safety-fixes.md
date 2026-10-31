# ADR 0007: two correctness/security bugs found in the native installer, fixed before "automatic" federation was attempted

## Status
Accepted (Milestone 6, third increment)

## Context
Asked to continue past ADR 0005/0006 toward *automatic* federation (Zorvok deciding on its own which sources to search, rather than a developer always passing `--source` by hand). Before building that, the actual multi-source resolution behavior `--native --source` already relies on was investigated directly in the installed `cocoapods`/`cocoapods-core` gem source — the same discipline ADR 0004/0005/0006 used. That investigation found two real bugs already present in the ADR 0005/0006 implementation, one functional and one a genuine security gap — not hypothetical, reproduced against real CocoaPods before being fixed.

### Bug 1 (functional): injecting a source silently drops the implicit default CDN
`Pod::Installer::Analyzer#sources` only falls back to the implicit default (`https://cdn.cocoapods.org/`) when `podfile.sources` is **empty**:

```ruby
elsif has_dependencies? && sources.empty? && plugin_sources.empty?
  sources = [Pod::TrunkSource::TRUNK_REPO_URL] + dependency_sources
```

The moment Zorvok's shim prepends its own `source` line (ADR 0005), `podfile.sources` is no longer empty, so this fallback never triggers. For the common case — a Podfile with no `source` lines of its own, which is most real CocoaPods projects — every *ordinary* pod in that Podfile would silently stop resolving the instant federation is used for any pod in the same project. Not caught earlier because every Milestone 6 test so far used a Podfile with exactly one pod, entirely federated — nothing exercised the "mixed" case until this review.

### Bug 2 (security): same pod, same version, two sources — CocoaPods picks one silently
Confirmed by direct reproduction, not just reading docs: two sources both hosting `SharedPod (1.0.0)`, declared together in a Podfile, installed with real `pod install`. Result: CocoaPods printed *"Found multiple specifications for `SharedPod (1.0.0)`"* — but still **exited 0** and picked the first-declared source, exactly matching `Specification::Set`'s own doc comment: *"If multiple sources have a specification for the required_version, the order in which they are provided is used to disambiguate."*

That's a warning a human might notice in interactive output; it is not something `zorvok install --native`'s exit code or Go error handling would ever see. Combined with ADR 0005's shim *prepending* its injected source before the Podfile's own content, Zorvok's own federated source would win ties over whatever else declares the same pod name — the exact dependency-confusion / package-takeover shape ARCHITECTURE.md §5 and ADR 0003 warned about, now concretely exploitable the moment two sources are both in play, which federation inherently requires. This is **not** "never a silent pick" (ADR 0003's own stated requirement).

## Decision
Both fixed in `engines/cocoapods/installer/zorvok_install.rb`, before any work on automatic (non-explicit) federation continued:

1. **Preserve the implicit default.** Before augmenting, parse the *original* (unaugmented) Podfile content once via `Pod::Podfile.from_ruby` and check `.sources`. If it's empty and Zorvok is about to inject a source, explicitly add the trunk CDN URL alongside it — reproducing exactly what CocoaPods would have done on its own. If the original Podfile already declared its own sources, add nothing beyond what `--source` asked for (respecting an explicit existing choice, not second-guessing it). Overridable via `ZORVOK_DEFAULT_SOURCE_URL` so this is testable hermetically against a local fixture instead of needing real network access to `cdn.cocoapods.org` in CI.
2. **Refuse ambiguity, don't warn-and-proceed.** Before calling `installer.install!`, construct a `Pod::Installer::Analyzer` the same way `install!` itself would (same `sandbox`/`podfile`/`lockfile`), call its real `#sources`, and for every dependency *without* an explicit `:source =>` pin, check how many of those resolved sources actually have a matching spec (`source.search(dep)`). More than one → abort loudly, before any download happens, naming the pod and every conflicting source's URL. A pod with an explicit per-pod `:source =>` pin is never ambiguous by construction and is skipped.

Both verified against real reproductions, not just unit-tested in isolation:
- `TestInstallNativeAbortsOnAmbiguousSource`: two real git-backed sources, same pod/version, no pin → `InstallNative` returns an error naming the pod and both source URLs; `Podfile.lock` is never written; the Podfile on disk is confirmed unchanged.
- `TestInstallNativePinnedSourceBypassesAmbiguityCheck`: the same two conflicting sources, but the dependency is explicitly pinned to one — installs successfully, proving the fix isn't overly broad.
- `TestInstallNativePreservesImplicitDefaultSource`: a Podfile with no `source` lines, one dependency resolvable only via the (test-substituted) implicit default and one resolvable only via an injected federated source — both install successfully, proving the default isn't silently dropped.

## Consequences
- Automatic (non-explicit-`--source`) federation remains explicitly **not built**. These fixes are a precondition for it, not a substitute — they make the *existing*, already-shipped `--native --source` mechanism safe to keep building on, but Zorvok still doesn't decide on its own which sources to search for which pods. That remains gated behind the dependency-confusion adversarial review ARCHITECTURE.md §5 calls for, which is a broader design question (how does Zorvok itself decide what counts as "the same pod" worth refusing on, across registries it doesn't control) than what this ADR's fixes address.
- The ambiguity check only runs inside the native installer path (`--native`); the default delegated `pod install` path (Milestone 2) is unaffected — it has always been subject to vanilla CocoaPods' warn-and-proceed behavior for multi-source Podfiles, same as any CocoaPods project today, federated or not.
- `ZORVOK_DEFAULT_SOURCE_URL` is a real, if minor, new surface: an environment variable that changes native-install behavior. Documented here and in the script's own comments so it's never a surprise; its only intended use is Zorvok's own test suite, though nothing stops a user from pointing their own private mirror at it if they want to.
