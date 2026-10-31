// Package registry is a Go client for the Zorvok registry's HTTP API
// (registry-server/api). Used by `zorvok login`/`trunk push`,
// `zorvok search`/`info`, and internal/publisher.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to one Zorvok registry's HTTP API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a Client for baseURL (e.g. "http://localhost:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError is returned for any non-2xx response, carrying the status
// code so callers (internal/publisher, internal/cli) can give specific
// guidance (409 = already published, 403 = not verified/not owner, ...).
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("registry: %s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("registry: HTTP %d", e.StatusCode)
}

type registerRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type registerResponse struct {
	SessionToken string `json:"session_token"`
	Email        string `json:"email"`
	Verified     bool   `json:"verified"`
}

// Register creates (or re-registers, for resending verification) an
// account and returns a fresh session token for this machine.
func (c *Client) Register(ctx context.Context, email, name string) (sessionToken string, err error) {
	var resp registerResponse
	if err := c.do(ctx, "POST", "/v1/auth/register", "", registerRequest{Email: email, Name: name}, &resp); err != nil {
		return "", err
	}
	return resp.SessionToken, nil
}

// PublishVersion publishes a podspec version. podspecJSON is the
// podspec's declarative JSON form (e.g. from `pod ipc spec`), submitted
// verbatim and never executed server-side.
type PublishedVersion struct {
	Package     string `json:"package"`
	Version     string `json:"version"`
	Checksum    string `json:"checksum"`
	PublishedBy string `json:"published_by"`
	PublishedAt string `json:"published_at"`
}

func (c *Client) PublishVersion(ctx context.Context, sessionToken, name, version, podspecJSON, source string) (*PublishedVersion, error) {
	req := struct {
		Version string `json:"version"`
		Podspec string `json:"podspec"`
		Source  string `json:"source"`
	}{version, podspecJSON, source}

	var resp PublishedVersion
	path := fmt.Sprintf("/v1/packages/%s/versions", name)
	if err := c.do(ctx, "POST", path, sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type PackageInfo struct {
	Name               string   `json:"name"`
	Owners             []string `json:"owners"`
	Deprecated         bool     `json:"deprecated"`
	DeprecationMessage string   `json:"deprecation_message"`
	Versions           []string `json:"versions"`
}

// GetPackage fetches a package's metadata — used by `zorvok info`.
func (c *Client) GetPackage(ctx context.Context, name string) (*PackageInfo, error) {
	var resp PackageInfo
	if err := c.do(ctx, "GET", "/v1/packages/"+name, "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Search queries the registry for packages matching query (a
// case-insensitive substring match, server-side — registry-server/api's
// handleSearch) — used by `zorvok search`.
func (c *Client) Search(ctx context.Context, query string) ([]PackageInfo, error) {
	var resp []PackageInfo
	path := "/v1/search?q=" + url.QueryEscape(query)
	if err := c.do(ctx, "GET", path, "", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Account is the authenticated identity behind a session token.
type Account struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	Verified  bool   `json:"verified"`
	CreatedAt string `json:"created_at"`
}

// Me returns the account authenticated by sessionToken — used by
// `zorvok trunk me`.
func (c *Client) Me(ctx context.Context, sessionToken string) (*Account, error) {
	var resp Account
	if err := c.do(ctx, "GET", "/v1/auth/me", sessionToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddOwner grants account publish rights on name — used by `zorvok
// trunk add-owner`. The caller must already be an owner.
func (c *Client) AddOwner(ctx context.Context, sessionToken, name, account string) error {
	req := struct {
		Email string `json:"email"`
	}{account}
	path := fmt.Sprintf("/v1/packages/%s/owners", name)
	return c.do(ctx, "POST", path, sessionToken, req, nil)
}

// RemoveOwner revokes account's publish rights on name — used by
// `zorvok trunk remove-owner`. The registry refuses to remove a
// package's last remaining owner.
func (c *Client) RemoveOwner(ctx context.Context, sessionToken, name, account string) error {
	req := struct {
		Email string `json:"email"`
	}{account}
	path := fmt.Sprintf("/v1/packages/%s/owners/remove", name)
	return c.do(ctx, "POST", path, sessionToken, req, nil)
}

// Deprecate marks name as deprecated, optionally with a message (e.g.
// "use OtherPod instead") — used by `zorvok trunk deprecate`.
func (c *Client) Deprecate(ctx context.Context, sessionToken, name, message string) error {
	req := struct {
		Message string `json:"message"`
	}{message}
	path := fmt.Sprintf("/v1/packages/%s/deprecate", name)
	return c.do(ctx, "POST", path, sessionToken, req, nil)
}

func (c *Client) do(ctx context.Context, method, path, sessionToken string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionToken != "" {
		req.Header.Set("Authorization", "Bearer "+sessionToken)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return &APIError{StatusCode: resp.StatusCode, Message: errBody.Error}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
