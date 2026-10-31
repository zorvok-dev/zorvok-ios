package packages

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore is an in-process Store, used for local development and
// tests. It is not persistent across restarts — see ROADMAP.md for the
// PostgreSQL implementation's status.
type MemoryStore struct {
	mu       sync.RWMutex
	packages map[string]*Package
	versions map[string]map[string]*Version // package name -> version -> Version
	audit    []AuditEvent
}

// NewMemoryStore returns an empty, ready-to-use MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		packages: map[string]*Package{},
		versions: map[string]map[string]*Version{},
	}
}

func (s *MemoryStore) record(actor, action, pkg, detail string) {
	s.audit = append(s.audit, AuditEvent{Time: time.Now().UTC(), Actor: actor, Action: action, Package: pkg, Detail: detail})
}

func (s *MemoryStore) GetOrCreate(_ context.Context, name, owner string) (*Package, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.packages[name]; ok {
		return p, nil
	}
	p := &Package{Name: name, Owners: []string{owner}, CreatedAt: time.Now().UTC()}
	s.packages[name] = p
	s.versions[name] = map[string]*Version{}
	s.record(owner, "package.create", name, "")
	return p, nil
}

func (s *MemoryStore) Get(_ context.Context, name string) (*Package, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.packages[name]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *MemoryStore) Search(_ context.Context, query string) ([]*Package, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	var results []*Package
	for _, p := range s.packages {
		if query == "" || strings.Contains(strings.ToLower(p.Name), query) {
			results = append(results, p)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

func (s *MemoryStore) IsOwner(_ context.Context, name, account string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.packages[name]
	if !ok {
		return false, ErrNotFound
	}
	for _, o := range p.Owners {
		if o == account {
			return true, nil
		}
	}
	return false, nil
}

func (s *MemoryStore) AddOwner(_ context.Context, name, account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packages[name]
	if !ok {
		return ErrNotFound
	}
	for _, o := range p.Owners {
		if o == account {
			return nil // already an owner
		}
	}
	p.Owners = append(p.Owners, account)
	s.record(account, "package.add_owner", name, account)
	return nil
}

func (s *MemoryStore) Deprecate(_ context.Context, name, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packages[name]
	if !ok {
		return ErrNotFound
	}
	p.Deprecated = true
	p.DeprecationMessage = message
	s.record("", "package.deprecate", name, message)
	return nil
}

func (s *MemoryStore) PublishVersion(_ context.Context, v Version) error {
	if err := ValidateName(v.PackageName); err != nil {
		return err
	}
	if err := ValidateVersion(v.Version); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.packages[v.PackageName]; !ok {
		return ErrNotFound
	}
	versions := s.versions[v.PackageName]
	if _, exists := versions[v.Version]; exists {
		return ErrVersionExists
	}
	v.PublishedAt = time.Now().UTC()
	versions[v.Version] = &v
	s.record(v.PublishedBy, "version.publish", v.PackageName, v.Version)
	return nil
}

func (s *MemoryStore) GetVersion(_ context.Context, name, version string) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.versions[name]
	if !ok {
		return nil, ErrNotFound
	}
	v, ok := versions[version]
	if !ok {
		return nil, ErrVersionNotFound
	}
	return v, nil
}

func (s *MemoryStore) ListVersions(_ context.Context, name string) ([]*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.versions[name]
	if !ok {
		return nil, ErrNotFound
	}
	result := make([]*Version, 0, len(versions))
	for _, v := range versions {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, nil
}

func (s *MemoryStore) AuditEvents(_ context.Context) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditEvent, len(s.audit))
	copy(out, s.audit)
	return out, nil
}

var _ Store = (*MemoryStore)(nil)
