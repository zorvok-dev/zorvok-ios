// Package safehttp holds the checks shared by every Zorvok HTTP client
// that sends a credential (a GitHub token, a registry session): where it
// may be sent, which redirects may be followed with it, and what text from
// a server is safe to print in a terminal.
package safehttp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// SecureURL reports whether u is https, or http to this machine
// (localhost, 127.0.0.0/8 or ::1, for a local dev registry or a test's
// fake), where nobody else is on the path. Those are the only places a
// token may be sent.
func SecureURL(u *url.URL) bool {
	if u == nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		host := strings.ToLower(u.Hostname())
		ip := net.ParseIP(host)
		return host == "localhost" || (ip != nil && ip.IsLoopback())
	}
	return false
}

// CheckRedirect is an http.Client CheckRedirect that refuses a redirect
// to anywhere SecureURL doesn't allow. Go resends the Authorization header
// on a redirect to the same host or one of its subdomains, even from https
// to plain http, so a misconfigured or hostile server could otherwise get
// the token sent unencrypted.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if !SecureURL(req.URL) {
		return fmt.Errorf("refusing to follow a redirect to %s, which isn't https", req.URL.Redacted())
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after %d redirects", len(via))
	}
	return nil
}

// NewClient returns an http.Client with timeout that only follows
// redirects CheckRedirect allows.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: CheckRedirect}
}

// Printable returns text from a server with control characters (other
// than newline and tab) and bidirectional-text controls removed, so
// printing it can't send escape sequences to the terminal or make it read
// differently from what it says.
func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)) {
			return -1
		}
		return r
	}, s)
}
