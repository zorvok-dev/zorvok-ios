package auth

import (
	"context"
	"errors"
	"testing"
)

func TestRegisterIssuesSessionButStaysUnverified(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	session, verifyToken, err := s.Register(ctx, "dev@example.com", "Dev")
	if err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || verifyToken == "" {
		t.Fatal("expected non-empty session and verify tokens")
	}
	if session.Token == verifyToken {
		t.Fatal("session token and verify token must not be the same value")
	}

	account, err := s.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if account.Verified {
		t.Fatal("a freshly registered account must not be verified yet")
	}
}

func TestVerifyEmailMarksAccountVerified(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	session, verifyToken, _ := s.Register(ctx, "dev@example.com", "Dev")

	if err := s.VerifyEmail(ctx, verifyToken); err != nil {
		t.Fatal(err)
	}

	account, err := s.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !account.Verified {
		t.Fatal("expected account to be verified")
	}
}

func TestVerifyEmailRejectsUnknownToken(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	err := s.VerifyEmail(ctx, "not-a-real-token")
	if !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("got %v, want ErrInvalidVerifyToken", err)
	}
}

func TestVerifyTokenIsSingleUse(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	_, verifyToken, _ := s.Register(ctx, "dev@example.com", "Dev")

	if err := s.VerifyEmail(ctx, verifyToken); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyEmail(ctx, verifyToken); !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("got %v, want ErrInvalidVerifyToken on reuse", err)
	}
}

func TestAuthenticateRejectsUnknownSession(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	_, err := s.Authenticate(ctx, "bogus")
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("got %v, want ErrInvalidSession", err)
	}
}
