// Package registrytest runs a real zorvok-registry binary as a
// subprocess for tests, since that server now lives in a separate
// repo. Mirrors the existing pattern of testing against a real
// `pod`/`git` on PATH: skip, don't fail, if the binary isn't there.
package registrytest

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"
	"time"
)

// Server is a running zorvok-registry subprocess.
type Server struct {
	BaseURL string

	mu  sync.Mutex
	log bytes.Buffer
}

func (s *Server) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.log.Write(p)
}

func (s *Server) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.log.String()
}

// RequireBinary skips the test if `zorvok-registry` isn't on PATH.
func RequireBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("zorvok-registry"); err != nil {
		t.Skip("skipping: no `zorvok-registry` binary on PATH (build it from zorvok-dev/zorvok-registry-ios and add it to PATH)")
	}
}

// Start launches a real zorvok-registry subprocess on a free local port
// and waits for it to accept requests. The process is killed
// automatically when the test finishes.
func Start(t *testing.T) *Server {
	t.Helper()
	RequireBinary(t)

	port, err := freePort()
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	srv := &Server{BaseURL: baseURL}

	cmd := exec.Command("zorvok-registry")
	cmd.Env = subprocessEnv("127.0.0.1:"+fmt.Sprint(port), baseURL)
	cmd.Stdout = srv
	cmd.Stderr = srv
	if err := cmd.Start(); err != nil {
		t.Fatalf("start zorvok-registry: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	if err := waitForReady(baseURL, 5*time.Second); err != nil {
		t.Fatalf("zorvok-registry did not become ready: %v\nlog:\n%s", err, srv.snapshot())
	}
	return srv
}

// subprocessEnv extends the test process's own environment rather
// than replacing it. cmd.Env = append(cmd.Env, ...) on a freshly
// constructed exec.Cmd starts from nil, so the previous code handed
// the subprocess exactly ZORVOK_REGISTRY_ADDR and
// ZORVOK_REGISTRY_BASE_URL and nothing else — no PATH, no HOME, no
// TMPDIR. zorvok-registry doesn't need any of those today, but a
// subprocess silently stripped of its environment is a trap for
// whatever it needs tomorrow.
func subprocessEnv(addr, baseURL string) []string {
	return append(os.Environ(),
		"ZORVOK_REGISTRY_ADDR="+addr,
		"ZORVOK_REGISTRY_BASE_URL="+baseURL,
	)
}

var verifyLinkRe = regexp.MustCompile(`verification email for (\S+): (\S+)`)

// WaitForVerifyLink polls the server's own log output (where
// auth.LogSender prints it, since there's no real mail transport) for
// the verification link emitted for email, matching how a developer
// using a local dev registry would find it themselves.
func (s *Server) WaitForVerifyLink(t *testing.T, email string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range verifyLinkRe.FindAllStringSubmatch(s.snapshot(), -1) {
			if m[1] == email {
				return m[2]
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no verification link found for %s in server output:\n%s", email, s.snapshot())
	return ""
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForReady(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/search", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s: %w", baseURL, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}
