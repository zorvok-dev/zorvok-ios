package api

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// withLogging logs each request's method, path, status, and duration —
// a minimal version of the audit-adjacent visibility brief §9 asks for
// at the transport layer (package-level audit events are tracked
// separately in packages.Store.AuditEvents).
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("[api] %s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// rateLimiter is a minimal fixed-window, per-client-IP limiter (brief
// §9: rate limiting / abuse protection). It intentionally avoids a
// token-bucket library dependency for something this simple; if burst
// smoothing ever matters, swap the implementation, not the interface.
type rateLimiter struct {
	limit  int
	window time.Duration

	mu     sync.Mutex
	counts map[string]*windowCount
}

type windowCount struct {
	windowStart time.Time
	count       int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, counts: map[string]*windowCount{}}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	wc, ok := rl.counts[key]
	if !ok || now.Sub(wc.windowStart) >= rl.window {
		rl.counts[key] = &windowCount{windowStart: now, count: 1}
		return true
	}
	if wc.count >= rl.limit {
		return false
	}
	wc.count++
	return true
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if !rl.allow(key) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded, try again shortly")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
