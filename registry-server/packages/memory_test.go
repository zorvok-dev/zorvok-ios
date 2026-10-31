package packages

import (
	"context"
	"errors"
	"testing"
)

func TestGetOrCreateIsIdempotentAndFirstOwnerWins(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	p1, err := s.GetOrCreate(ctx, "Alamofire", "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.GetOrCreate(ctx, "Alamofire", "mallory@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Owners) != 1 || p2.Owners[0] != "alice@example.com" {
		t.Fatalf("second GetOrCreate must not change ownership, got %v", p2.Owners)
	}
	if p1 != p2 {
		t.Fatal("expected the same package record both times")
	}
}

func TestPublishVersionRejectsDuplicateImmutably(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.GetOrCreate(ctx, "Alamofire", "alice@example.com")

	v := Version{PackageName: "Alamofire", Version: "1.0.0", PublishedBy: "alice@example.com"}
	if err := s.PublishVersion(ctx, v); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	err := s.PublishVersion(ctx, v)
	if !errors.Is(err, ErrVersionExists) {
		t.Fatalf("got %v, want ErrVersionExists", err)
	}

	// Even a "republish" with different content for the same version
	// must be rejected — that's the whole point of immutability.
	v2 := v
	v2.Podspec = `{"changed": true}`
	if err := s.PublishVersion(ctx, v2); !errors.Is(err, ErrVersionExists) {
		t.Fatalf("got %v, want ErrVersionExists even with different content", err)
	}
}

func TestPublishVersionRequiresExistingPackage(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	err := s.PublishVersion(ctx, Version{PackageName: "Nope", Version: "1.0.0"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestIsOwner(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.GetOrCreate(ctx, "Alamofire", "alice@example.com")

	ok, err := s.IsOwner(ctx, "Alamofire", "alice@example.com")
	if err != nil || !ok {
		t.Fatalf("got (%v, %v), want (true, nil)", ok, err)
	}
	ok, err = s.IsOwner(ctx, "Alamofire", "mallory@example.com")
	if err != nil || ok {
		t.Fatalf("got (%v, %v), want (false, nil)", ok, err)
	}
}

func TestAddOwnerIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.GetOrCreate(ctx, "Alamofire", "alice@example.com")

	if err := s.AddOwner(ctx, "Alamofire", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddOwner(ctx, "Alamofire", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Get(ctx, "Alamofire")
	if len(p.Owners) != 2 {
		t.Fatalf("expected 2 owners, got %v", p.Owners)
	}
}

func TestSearchIsCaseInsensitiveSubstring(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.GetOrCreate(ctx, "Alamofire", "alice@example.com")
	s.GetOrCreate(ctx, "SDWebImage", "alice@example.com")

	results, err := s.Search(ctx, "ala")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Alamofire" {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestListVersionsSortedAscending(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	s.GetOrCreate(ctx, "Alamofire", "alice@example.com")
	s.PublishVersion(ctx, Version{PackageName: "Alamofire", Version: "2.0.0", PublishedBy: "alice@example.com"})
	s.PublishVersion(ctx, Version{PackageName: "Alamofire", Version: "1.0.0", PublishedBy: "alice@example.com"})

	versions, err := s.ListVersions(ctx, "Alamofire")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != "1.0.0" || versions[1].Version != "2.0.0" {
		t.Fatalf("unexpected order: %v", versions)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"Alamofire", "SD_WebImage", "pod-name", "A", "Pod++"}
	invalid := []string{"", "1Pod", "-pod", "pod/name", "pod name"}
	for _, n := range valid {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", n, err)
		}
	}
	for _, n := range invalid {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", n)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	valid := []string{"1.0.0", "0.0.1", "1.2.3-beta.1", "1.2.3+build.5"}
	invalid := []string{"", "1.0", "v1.0.0", "1.0.0.0", "latest"}
	for _, v := range valid {
		if err := ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
	for _, v := range invalid {
		if err := ValidateVersion(v); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error", v)
		}
	}
}
