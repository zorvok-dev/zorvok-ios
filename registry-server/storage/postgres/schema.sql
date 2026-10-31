-- Zorvok registry PostgreSQL schema (ADR 0003: PostgreSQL for metadata,
-- object storage for artifacts — this file is metadata only).
--
-- STATUS: design artifact, not yet wired to a running implementation of
-- packages.Store/auth.Store, and not exercised against a real database
-- in this session (the local Docker daemon wasn't running — see
-- ROADMAP.md). Treat this as a reviewable schema proposal, not a tested
-- migration. The relational constraints below (UNIQUE on package+version,
-- FK ownership) are exactly the DB-level enforcement ARCHITECTURE.md §2.4
-- calls for on top of the application-level checks already implemented
-- in registry-server/packages and registry-server/auth's in-memory store.

CREATE TABLE accounts (
    email        TEXT PRIMARY KEY,
    name         TEXT NOT NULL DEFAULT '',
    verified     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token        TEXT PRIMARY KEY,
    email        TEXT NOT NULL REFERENCES accounts(email) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE verification_tokens (
    token        TEXT PRIMARY KEY,
    email        TEXT NOT NULL REFERENCES accounts(email) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE packages (
    name                  TEXT PRIMARY KEY,
    deprecated            BOOLEAN NOT NULL DEFAULT FALSE,
    deprecation_message   TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ownership as its own table, not an array column on packages, so a
-- unique constraint can prevent duplicate (package, owner) rows and a
-- foreign key can enforce that only a real account can own a package —
-- an array column can't express either.
CREATE TABLE package_owners (
    package_name   TEXT NOT NULL REFERENCES packages(name) ON DELETE CASCADE,
    email          TEXT NOT NULL REFERENCES accounts(email) ON DELETE RESTRICT,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (package_name, email)
);

-- The UNIQUE(package_name, version) constraint is the database-level
-- enforcement of version immutability (ARCHITECTURE.md §2.4) — a
-- stronger guarantee than the application-level check in
-- packages.MemoryStore.PublishVersion alone, since it holds even under
-- concurrent writes or a future code path that forgets the check.
CREATE TABLE versions (
    id              BIGSERIAL PRIMARY KEY,
    package_name    TEXT NOT NULL REFERENCES packages(name) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    podspec         JSONB NOT NULL,      -- stored verbatim, never executed (brief §9)
    source          TEXT NOT NULL DEFAULT '', -- a :git/:http reference; reference-only hosting per ADR 0003
    checksum        TEXT NOT NULL,       -- SHA-256 of podspec, computed server-side
    published_by    TEXT NOT NULL REFERENCES accounts(email) ON DELETE RESTRICT,
    published_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    downloads       BIGINT NOT NULL DEFAULT 0,
    UNIQUE (package_name, version)
);

CREATE INDEX versions_package_name_idx ON versions (package_name);

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor       TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,
    package_name TEXT,
    detail      TEXT NOT NULL DEFAULT ''
);
