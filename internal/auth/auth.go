// Package auth stores Zorvok's local session tokens on disk — one per
// registry host, no password, mirroring CocoaPods Trunk's model.
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zorvok-dev/zorvok-ios/internal/config"
)

// Credentials holds the locally stored session for one registry host.
type Credentials struct {
	Email string `yaml:"email"`
	Token string `yaml:"token"`
}

// Store is a map of registry host -> Credentials, persisted as one file.
type Store struct {
	sessions map[string]Credentials
	removed  map[string]bool // hosts explicitly removed since Load, applied by Save
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

	s := &Store{sessions: map[string]Credentials{}, removed: map[string]bool{}, path: path}

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

// Save persists the store to disk with owner-only permissions, since
// it holds session tokens.
//
// It merges this Store's changes (every Set/Remove since Load) onto
// whatever the file currently holds, rather than overwriting the
// whole file with this process's in-memory snapshot — two `zorvok
// login` calls to different registries running close together must
// not have one silently erase the other's just-written session, which
// a blind overwrite would do (last Save wins, full stop). The write
// itself is atomic: built in a temp file in the same directory, then
// renamed into place, so a crash or kill mid-write can't truncate or
// corrupt the one file every registry's session lives in.
func (s *Store) Save() error {
	if s.path == "" {
		return fmt.Errorf("credential store has no associated path")
	}

	onDisk := map[string]Credentials{}
	if data, err := os.ReadFile(s.path); err == nil {
		// Best-effort: a concurrently-corrupted file just means
		// nothing to merge, not a reason to fail this Save.
		yaml.Unmarshal(data, &onDisk)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", s.path, err)
	}
	for host, c := range s.sessions {
		onDisk[host] = c
	}
	for host := range s.removed {
		delete(onDisk, host)
	}

	data, err := yaml.Marshal(onDisk)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", s.path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("set permissions on %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace %s: %w", s.path, err)
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
	delete(s.removed, host)
}

// Remove deletes stored credentials for host.
func (s *Store) Remove(host string) {
	delete(s.sessions, host)
	if s.removed == nil {
		s.removed = map[string]bool{}
	}
	s.removed[host] = true
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
