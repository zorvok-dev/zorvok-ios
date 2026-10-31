# registry-server/storage

Holds persistence-backend implementations of the `packages.Store` and
`auth.Store` interfaces.

## Status

- **In-memory** (`registry-server/packages.MemoryStore`,
  `registry-server/auth.MemoryStore`): implemented, tested, used by
  `cmd/zorvok-registry` today. Not persistent across restarts — local
  development and the Milestone 3 API test suite only.
- **PostgreSQL** (`postgres/schema.sql`): schema designed per ADR 0003,
  but **not yet implemented or tested against a real database**. This
  session's local Docker daemon wasn't running, so a `database/sql`
  implementation satisfying `packages.Store`/`auth.Store` against this
  schema is deliberately deferred rather than shipped unverified — see
  ROADMAP.md Milestone 3. The schema itself is still a useful, reviewable
  design artifact: it's where the DB-level immutability/ownership
  constraints ARCHITECTURE.md §2.4 calls for would live (e.g.
  `UNIQUE(package_name, version)`), on top of the application-level
  checks already enforced in the in-memory store.
- **Object storage / CDN** (artifacts, not metadata): not started.
  ARCHITECTURE.md §5.5 scopes Milestone 3 to reference-only package
  hosting (a podspec's `source` is a `:git`/`:http` URL, like CocoaPods
  today) — no artifact upload/storage is needed yet.

## Next step

Implement a `postgres.Store` (satisfying both `packages.Store` and
`auth.Store`) against `schema.sql`, with a `docker-compose.yml` for local
Postgres (brief's engineering rule: "Docker-based local development for
backend services") and tests that skip (not fail) when no reachable
database is configured — matching the pattern already used for the
CocoaPods compatibility tests skipping without a local `pod` install.
