// Package api implements the Zorvok registry's versioned HTTP API
// (ARCHITECTURE.md §2.4). Milestone 3 scope: package/version metadata,
// search, and account registration/verification — server-side only. No
// CLI wiring yet (`zorvok login`/`trunk push` are Milestone 4); this
// package is validated directly via HTTP tests (ROADMAP.md).
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zorvok-dev/zorvok/registry-server/auth"
	"github.com/zorvok-dev/zorvok/registry-server/packages"
)

// Server holds the registry's dependencies and builds the HTTP handler.
type Server struct {
	Packages packages.Store
	Accounts auth.Store
	Sender   auth.EmailSender
	// BaseURL is used to build the verification link sent to new
	// accounts (e.g. "http://localhost:8080"). Required.
	BaseURL string

	limiter *rateLimiter
}

// NewServer constructs a Server with sane defaults (a per-IP rate
// limiter — brief §9/§5: rate limiting, abuse protection).
func NewServer(pkgs packages.Store, accounts auth.Store, sender auth.EmailSender, baseURL string) *Server {
	return &Server{
		Packages: pkgs,
		Accounts: accounts,
		Sender:   sender,
		BaseURL:  strings.TrimSuffix(baseURL, "/"),
		limiter:  newRateLimiter(60, time.Minute), // 60 requests/min/IP
	}
}

// Routes builds the HTTP handler, wrapping it in logging and
// rate-limiting middleware.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.HandleFunc("GET /v1/auth/verify", s.handleVerify)

	mux.HandleFunc("GET /v1/search", s.handleSearch)
	mux.HandleFunc("GET /v1/packages/{name}", s.handleGetPackage)
	mux.HandleFunc("GET /v1/packages/{name}/versions/{version}", s.handleGetVersion)
	mux.HandleFunc("POST /v1/packages/{name}/versions", s.handlePublishVersion)
	mux.HandleFunc("POST /v1/packages/{name}/owners", s.handleAddOwner)
	mux.HandleFunc("POST /v1/packages/{name}/deprecate", s.handleDeprecate)

	return withLogging(s.limiter.middleware(mux))
}

// --- request/response JSON shapes ---

type registerRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type registerResponse struct {
	SessionToken string `json:"session_token"`
	Email        string `json:"email"`
	Verified     bool   `json:"verified"`
}

type publishRequest struct {
	Version string `json:"version"`
	Podspec string `json:"podspec"` // the podspec as JSON (e.g. `pod ipc spec` output), stored verbatim
	Source  string `json:"source"`  // a :git/:http reference, per ADR 0003 (reference-only hosting)
}

type packageResponse struct {
	Name               string   `json:"name"`
	Owners             []string `json:"owners"`
	Deprecated         bool     `json:"deprecated"`
	DeprecationMessage string   `json:"deprecation_message,omitempty"`
	Versions           []string `json:"versions"`
}

type versionResponse struct {
	Package     string `json:"package"`
	Version     string `json:"version"`
	Podspec     string `json:"podspec"`
	Source      string `json:"source"`
	Checksum    string `json:"checksum"`
	PublishedBy string `json:"published_by"`
	PublishedAt string `json:"published_at"`
}

// --- handlers ---

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}

	session, verifyToken, err := s.Accounts.Register(r.Context(), req.Email, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	verifyURL := fmt.Sprintf("%s/v1/auth/verify?token=%s", s.BaseURL, verifyToken)
	if err := s.Sender.Send(r.Context(), req.Email, verifyURL); err != nil {
		log.Printf("[api] failed to send verification email to %s: %v", req.Email, err)
	}

	writeJSON(w, http.StatusCreated, registerResponse{SessionToken: session.Token, Email: req.Email, Verified: false})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}
	if err := s.Accounts.VerifyEmail(r.Context(), token); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired verification token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results, err := s.Packages.Search(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	out := make([]packageResponse, 0, len(results))
	for _, p := range results {
		out = append(out, s.toPackageResponse(r.Context(), p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetPackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	p, err := s.Packages.Get(r.Context(), name)
	if err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toPackageResponse(r.Context(), p))
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	v, err := s.Packages.GetVersion(r.Context(), name, version)
	if err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVersionResponse(v))
}

// handlePublishVersion implements the publishing workflow in
// ARCHITECTURE.md §2.4/brief §6: authenticate, require verified email,
// validate the podspec structurally (never execute it — brief §9),
// compute the checksum server-side, enforce ownership (first pusher
// becomes owner) and immutability (rejected by the store itself too).
func (s *Server) handlePublishVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := packages.ValidateName(name); err != nil {
		writeError(w, http.StatusBadRequest, "invalid package name")
		return
	}

	account, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization bearer token")
		return
	}
	if !account.Verified {
		writeError(w, http.StatusForbidden, "verify your email before publishing (see the link sent on registration)")
		return
	}

	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := packages.ValidateVersion(req.Version); err != nil {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	if err := validatePodspec(req.Podspec, name, req.Version); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Ownership: creates the package (owned by this account) on first
	// publish, or requires existing ownership on subsequent ones —
	// brief §9: protection against package takeover.
	pkg, err := s.Packages.GetOrCreate(r.Context(), name, account.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve package")
		return
	}
	isOwner := false
	for _, o := range pkg.Owners {
		if o == account.Email {
			isOwner = true
			break
		}
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "you are not an owner of this package")
		return
	}

	sum := sha256.Sum256([]byte(req.Podspec))
	checksum := hex.EncodeToString(sum[:])

	v := packages.Version{
		PackageName: name,
		Version:     req.Version,
		Podspec:     req.Podspec,
		Source:      req.Source,
		Checksum:    checksum,
		PublishedBy: account.Email,
	}
	if err := s.Packages.PublishVersion(r.Context(), v); err != nil {
		if errors.Is(err, packages.ErrVersionExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "publish failed")
		return
	}

	published, _ := s.Packages.GetVersion(r.Context(), name, req.Version)
	writeJSON(w, http.StatusCreated, toVersionResponse(published))
}

func (s *Server) handleAddOwner(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	account, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization bearer token")
		return
	}
	isOwner, err := s.Packages.IsOwner(r.Context(), name, account.Email)
	if err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "you are not an owner of this package")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeError(w, http.StatusBadRequest, "a new owner email is required")
		return
	}
	if err := s.Packages.AddOwner(r.Context(), name, req.Email); err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeprecate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	account, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization bearer token")
		return
	}
	isOwner, err := s.Packages.IsOwner(r.Context(), name, account.Email)
	if err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "you are not an owner of this package")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := s.Packages.Deprecate(r.Context(), name, req.Message); err != nil {
		writeNotFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- helpers ---

func (s *Server) authenticate(r *http.Request) (*auth.Account, bool) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return nil, false
	}
	account, err := s.Accounts.Authenticate(r.Context(), token)
	if err != nil {
		return nil, false
	}
	return account, true
}

func (s *Server) toPackageResponse(ctx context.Context, p *packages.Package) packageResponse {
	versions, _ := s.Packages.ListVersions(ctx, p.Name)
	versionStrings := make([]string, 0, len(versions))
	for _, v := range versions {
		versionStrings = append(versionStrings, v.Version)
	}
	return packageResponse{
		Name:               p.Name,
		Owners:             p.Owners,
		Deprecated:         p.Deprecated,
		DeprecationMessage: p.DeprecationMessage,
		Versions:           versionStrings,
	}
}

func toVersionResponse(v *packages.Version) versionResponse {
	return versionResponse{
		Package:     v.PackageName,
		Version:     v.Version,
		Podspec:     v.Podspec,
		Source:      v.Source,
		Checksum:    v.Checksum,
		PublishedBy: v.PublishedBy,
		PublishedAt: v.PublishedAt.Format(time.RFC3339),
	}
}

// validatePodspec applies the only server-side podspec validation
// Milestone 3 does: structural checks on the declarative JSON, never
// executing it (brief §9: "never execute arbitrary podspec or package
// scripts on the registry server"). Deeper linting (`pod lib lint`'s
// trial build) is and stays client-side — ARCHITECTURE.md §2.4.
func validatePodspec(podspec, expectedName, expectedVersion string) error {
	if strings.TrimSpace(podspec) == "" {
		return fmt.Errorf("podspec is required")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(podspec), &decoded); err != nil {
		return fmt.Errorf("podspec must be valid JSON (e.g. `pod ipc spec` output): %w", err)
	}
	if name, _ := decoded["name"].(string); name != expectedName {
		return fmt.Errorf("podspec name %q does not match package %q", name, expectedName)
	}
	if version, _ := decoded["version"].(string); version != expectedVersion {
		return fmt.Errorf("podspec version %q does not match requested version %q", version, expectedVersion)
	}
	return nil
}

func writeNotFoundOr500(w http.ResponseWriter, err error) {
	if errors.Is(err, packages.ErrNotFound) || errors.Is(err, packages.ErrVersionNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
