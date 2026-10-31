package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-process Store for local development and tests.
// Not persistent across restarts.
type MemoryStore struct {
	mu           sync.RWMutex
	accounts     map[string]*Account // email -> account
	sessions     map[string]string   // session token -> email
	verifyTokens map[string]string   // verify token -> email
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		accounts:     map[string]*Account{},
		sessions:     map[string]string{},
		verifyTokens: map[string]string{},
	}
}

func (s *MemoryStore) Register(_ context.Context, email, name string) (Session, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.accounts[email]; !ok {
		s.accounts[email] = &Account{Email: email, Name: name, CreatedAt: time.Now().UTC()}
	}

	sessionToken := randomToken()
	s.sessions[sessionToken] = email

	verifyToken := randomToken()
	s.verifyTokens[verifyToken] = email

	return Session{Token: sessionToken, Email: email, CreatedAt: time.Now().UTC()}, verifyToken, nil
}

func (s *MemoryStore) VerifyEmail(_ context.Context, verifyToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	email, ok := s.verifyTokens[verifyToken]
	if !ok {
		return ErrInvalidVerifyToken
	}
	delete(s.verifyTokens, verifyToken)

	account, ok := s.accounts[email]
	if !ok {
		return ErrAccountNotFound
	}
	account.Verified = true
	return nil
}

func (s *MemoryStore) Authenticate(_ context.Context, sessionToken string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	email, ok := s.sessions[sessionToken]
	if !ok {
		return nil, ErrInvalidSession
	}
	account, ok := s.accounts[email]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *MemoryStore) Get(_ context.Context, email string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[email]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

var _ Store = (*MemoryStore)(nil)
