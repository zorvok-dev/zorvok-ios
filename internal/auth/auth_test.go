package auth

import "testing"

func TestRedactToken(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"abc":          "***",
		"abcdef":       "******",
		"abcdefgh":     "abcdef…",
		"abcdefghijkl": "abcdef…",
	}
	for in, want := range cases {
		if got := RedactToken(in); got != want {
			t.Errorf("RedactToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStoreSetGetRemove(t *testing.T) {
	s := &Store{sessions: map[string]Credentials{}}

	if _, ok := s.Get("registry.zorvok.dev"); ok {
		t.Fatal("expected no credentials initially")
	}

	s.Set("registry.zorvok.dev", Credentials{Email: "dev@example.com", Token: "abcdef123456"})
	c, ok := s.Get("registry.zorvok.dev")
	if !ok || c.Email != "dev@example.com" {
		t.Fatalf("got (%+v, %v)", c, ok)
	}

	if hosts := s.Hosts(); len(hosts) != 1 || hosts[0] != "registry.zorvok.dev" {
		t.Fatalf("unexpected hosts: %v", hosts)
	}

	s.Remove("registry.zorvok.dev")
	if _, ok := s.Get("registry.zorvok.dev"); ok {
		t.Fatal("expected credentials to be removed")
	}
}
