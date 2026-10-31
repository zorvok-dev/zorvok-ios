// Command zorvok-registry runs the Zorvok package registry's HTTP API
// (registry-server). Milestone 3 ships only the in-memory store — see
// ROADMAP.md for the PostgreSQL backend's status. This binary is for
// local development and testing the API directly; it is not the
// production deployment story, which still needs the PostgreSQL +
// object storage + CDN stack from ADR 0003.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/zorvok-dev/zorvok/registry-server/api"
	"github.com/zorvok-dev/zorvok/registry-server/auth"
	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

func main() {
	addr := envOr("ZORVOK_REGISTRY_ADDR", ":8080")
	baseURL := envOr("ZORVOK_REGISTRY_BASE_URL", "http://localhost:8080")

	server := api.NewServer(
		packages.NewMemoryStore(),
		auth.NewMemoryStore(),
		auth.LogSender{}, // no real email transport yet — verification links are logged, see ROADMAP.md
		baseURL,
	)

	log.Printf("zorvok-registry listening on %s (base URL %s)", addr, baseURL)
	log.Printf("WARNING: in-memory store — all data is lost on restart. PostgreSQL backend is not yet wired, see ROADMAP.md.")
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
