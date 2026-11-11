package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Sign-in limits: failed attempts allowed per window, per username and per
// address, before further attempts are refused until the window passes.
const (
	attemptWindow  = 15 * time.Minute
	failsPerUser   = 10
	failsPerAddr   = 30
	attemptKeysCap = 10_000
)

// attempts remembers recent failed sign-ins and registrations.
type attempts struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

func newAttempts() *attempts {
	return &attempts{fails: map[string][]time.Time{}}
}

// limit is how many failures a key may have in the window.
func limit(key string) int {
	if strings.HasPrefix(key, "user:") {
		return failsPerUser
	}
	return failsPerAddr
}

// blocked reports whether any key has used up its failures.
func (a *attempts) blocked(now time.Time, keys ...string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, k := range keys {
		if len(a.recent(now, k)) >= limit(k) {
			return true
		}
	}
	return false
}

// fail records a failed attempt against each key.
func (a *attempts) fail(now time.Time, keys ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.fails) > attemptKeysCap {
		a.sweep(now)
	}
	for _, k := range keys {
		a.fails[k] = append(a.recent(now, k), now)
	}
}

// clear forgets a key's failures (after a successful sign-in).
func (a *attempts) clear(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.fails, key)
}

// recent drops failures older than the window. Callers hold the lock.
func (a *attempts) recent(now time.Time, key string) []time.Time {
	kept := a.fails[key][:0]
	for _, t := range a.fails[key] {
		if now.Sub(t) < attemptWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(a.fails, key)
		return nil
	}
	a.fails[key] = kept
	return kept
}

// sweep drops every expired key. Callers hold the lock.
func (a *attempts) sweep(now time.Time) {
	for k := range a.fails {
		a.recent(now, k)
	}
}

// clientAddr is the caller's address: the one the reverse proxy saw (the last
// X-Forwarded-For entry, which the proxy appends) when it is trusted, else
// the connection's.
func (s *Server) clientAddr(r *http.Request) string {
	if s.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// contentPolicy allows the app's own scripts and media, Google's fonts, and
// the blob audio of narration, and nothing else.
const contentPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// securityHeaders hardens every response: no sniffing, no framing, a strict
// content policy, and HSTS once the site is served over HTTPS.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", contentPolicy)
		if overHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// healthz tells a container runtime the server is up.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok\n"))
}
