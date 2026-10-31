// Package auth manages Zorvok's local credential storage.
//
// This is a Milestone 1 foundation only: it defines how a session token
// is stored and loaded on disk. It intentionally does not implement
// `zorvok login` itself (no registry API exists yet to authenticate
// against) nor Trunk-style email verification — those land with the
// registry and publishing milestones (M3-M4). The storage format mirrors
// CocoaPods Trunk's model: a per-machine session token, no password.
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zorvok-dev/zorvok/internal/config"
)

// Credentials holds the locally stored session for one registry host.
type Credentials struct {
	Email string `yaml:"email"`
	Token string `yaml:"token"`
}

// Store is a map of registry host -> Credentials, persisted as one file.
type Store struct {
	sessions map[string]Credentials
	path     string
}

func credentialsPath() (string, error) {
	dir, err := config.GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.yml"), nil
}

// LoadStore reads the local credential store. A missing file yields an
// empty, usable store.
func LoadStore() (*Store, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	s := &Store{sessions: map[string]Credentials{}, path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &s.sessions); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if s.sessions == nil {
		s.sessions = map[string]Credentials{}
	}
	return s, nil
}

// Save persists the store to disk with owner-only permissions, since it
// holds session tokens.
func (s *Store) Save() error {
	data, err := yaml.Marshal(s.sessions)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", s.path, err)
	}
	return nil
}

// Get returns the stored credentials for host, if any.
func (s *Store) Get(host string) (Credentials, bool) {
	c, ok := s.sessions[host]
	return c, ok
}

// Set stores credentials for host.
func (s *Store) Set(host string, c Credentials) {
	s.sessions[host] = c
}

// Remove deletes stored credentials for host.
func (s *Store) Remove(host string) {
	delete(s.sessions, host)
}

// Hosts lists every host with stored credentials.
func (s *Store) Hosts() []string {
	hosts := make([]string, 0, len(s.sessions))
	for h := range s.sessions {
		hosts = append(hosts, h)
	}
	return hosts
}

// RedactToken returns a short, display-safe form of a token
// (e.g. "a1b2c3…", never the full value) for use in `zorvok config`/`doctor`
// output.
func RedactToken(token string) string {
	if len(token) <= 6 {
		return strings.Repeat("*", len(token))
	}
	return token[:6] + "…"
}
