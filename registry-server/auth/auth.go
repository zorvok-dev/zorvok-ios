// Package auth implements the registry's publisher accounts, mirroring
// CocoaPods Trunk's UX (ARCHITECTURE.md §2.4): email-verified, per-machine
// session tokens, no passwords. Registering issues a session token
// immediately so the CLI can store it right away, but publishing is
// blocked until the account's email is verified — exactly like `pod
// trunk register` followed by clicking the emailed confirmation link.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrInvalidSession     = errors.New("invalid or expired session token")
	ErrInvalidVerifyToken = errors.New("invalid or expired verification token")
	ErrNotVerified        = errors.New("account email not verified yet")
)

// Account is a publisher identity.
type Account struct {
	Email     string
	Name      string
	Verified  bool
	CreatedAt time.Time
}

// Session is one machine's authentication credential for an Account.
type Session struct {
	Token     string
	Email     string
	CreatedAt time.Time
}

// EmailSender delivers the verification link. Milestone 3 ships only
// LogSender (see sender.go) — no real email transport is implemented
// yet; this is a known, documented limitation (ROADMAP.md), not a
// silent gap, since the interface makes a real provider a drop-in
// addition later without touching Store or the HTTP layer.
type EmailSender interface {
	Send(ctx context.Context, to, verifyURL string) error
}

// Store is the persistence boundary for accounts/sessions. Milestone 3
// ships an in-memory implementation; see ROADMAP.md for PostgreSQL
// status.
type Store interface {
	// Register creates the account if it doesn't exist yet (re-registering
	// an existing, unverified account is allowed — it just issues a new
	// session and verification token, matching Trunk's own behavior of
	// letting you resend verification). It always returns a fresh
	// session token for this call's local machine.
	Register(ctx context.Context, email, name string) (session Session, verifyToken string, err error)
	VerifyEmail(ctx context.Context, verifyToken string) error
	Authenticate(ctx context.Context, sessionToken string) (*Account, error)
	Get(ctx context.Context, email string) (*Account, error)
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing is unrecoverable
	}
	return hex.EncodeToString(b)
}
