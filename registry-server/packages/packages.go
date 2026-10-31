// Package packages is the Zorvok registry's core domain: package
// metadata, versions, and ownership (ARCHITECTURE.md §2.4). Version
// records live here rather than in a separate package — they're one
// aggregate (immutability and ownership checks need both together), so
// splitting them per the brief's suggested repo layout would just be
// import ceremony with no present benefit; registry-server/versions/ is
// intentionally not used (see ARCHITECTURE.md's structure note).
package packages

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("package not found")
	ErrVersionNotFound = errors.New("version not found")
	// ErrVersionExists signals a Milestone-3 security invariant: a
	// published version is immutable (ARCHITECTURE.md §2.4, brief §9).
	ErrVersionExists  = errors.New("version already published: published versions are immutable")
	ErrNotOwner       = errors.New("account is not an owner of this package")
	ErrInvalidName    = errors.New("invalid package name")
	ErrInvalidVersion = errors.New("invalid version")
)

// Package is a registered pod name and its ownership/deprecation state.
// Per ADR 0003, a pod's name is its registry identifier, matching
// CocoaPods' own convention.
type Package struct {
	Name               string
	Owners             []string // account emails authorized to publish new versions
	Deprecated         bool
	DeprecationMessage string
	CreatedAt          time.Time
}

// Version is one immutable, published release of a Package.
//
// Source is a reference (a :git/:http URL, exactly like a CocoaPods
// podspec's `s.source`), not an uploaded artifact — ADR 0003 scopes
// Milestone 3 to reference-only hosting; Zorvok-hosted artifact storage
// is a later, opt-in enhancement.
type Version struct {
	PackageName string
	Version     string
	Podspec     string // the podspec as submitted (JSON), stored verbatim, never executed server-side
	Source      string
	Checksum    string // SHA-256 of Podspec, computed server-side on publish
	PublishedBy string // account email
	PublishedAt time.Time
	Downloads   int64
}

// AuditEvent is one recorded registry mutation (brief §9: audit logs).
type AuditEvent struct {
	Time    time.Time
	Actor   string
	Action  string
	Package string
	Detail  string
}

// Store is the persistence boundary for package/version data. Milestone
// 3 ships an in-memory implementation (memory.go); a PostgreSQL
// implementation is designed (see registry-server/storage/postgres) but
// not yet built against a real database in this session — see
// ROADMAP.md.
type Store interface {
	// GetOrCreate returns the package, creating it with owner as its
	// sole owner if it doesn't exist yet. This is the "first pusher
	// becomes owner" rule (brief §6, §9: protection against package
	// takeover) — called only from a publish flow that has already
	// authenticated owner.
	GetOrCreate(ctx context.Context, name, owner string) (*Package, error)
	Get(ctx context.Context, name string) (*Package, error)
	Search(ctx context.Context, query string) ([]*Package, error)
	IsOwner(ctx context.Context, name, account string) (bool, error)
	AddOwner(ctx context.Context, name, account string) error
	Deprecate(ctx context.Context, name, message string) error

	// PublishVersion fails with ErrVersionExists if (name, version) is
	// already published — enforcing immutability at the store layer,
	// not just in the HTTP handler, so no code path can bypass it.
	PublishVersion(ctx context.Context, v Version) error
	GetVersion(ctx context.Context, name, version string) (*Version, error)
	ListVersions(ctx context.Context, name string) ([]*Version, error)

	AuditEvents(ctx context.Context) ([]AuditEvent, error)
}
