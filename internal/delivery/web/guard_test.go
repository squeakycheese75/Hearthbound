package web_test

import (
	"dnd/internal/delivery/web"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFailedSignInsAreThrottled(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	for i := range 10 {
		if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"James","password":"wrong"}`, ""); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, res.StatusCode)
		}
	}
	res, body := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"james","password":"pw"}`, "")
	if res.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, "too many failed sign-ins") {
		t.Fatalf("the 11th try is refused, even with the right password: %d %s", res.StatusCode, body)
	}
	// The address has budget left for other accounts, up to its own limit.
	for i := range 20 {
		if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", fmt.Sprintf(`{"username":"user%d","password":"x"}`, i), ""); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("other accounts: attempt %d: %d", i, res.StatusCode)
		}
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"someone","password":"x"}`, ""); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("30 failures from one address: %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/register", `{"invite":"inv","username":"new","password":"pw"}`, ""); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a throttled address can't register either: %d", res.StatusCode)
	}
}

func TestGuessingInvitesIsThrottled(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	for range 30 {
		_, _ = do(t, "POST", r.srv.URL+"/api/auth/register", `{"invite":"guess","username":"x","password":"y"}`, "")
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/register", `{"invite":"inv","username":"x","password":"y"}`, ""); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("after 30 bad invites: %d", res.StatusCode)
	}
}

func TestBehindAProxyTheForwardedAddressCounts(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, func(s *web.Server) { s.TrustProxy = true })
	for i := range 30 {
		_, _ = do(t, "POST", r.srv.URL+"/api/auth/login", fmt.Sprintf(`{"username":"u%d","password":"x"}`, i), "", "X-Forwarded-For", "6.6.6.6, 203.0.113.9")
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"james","password":"pw"}`, "", "X-Forwarded-For", "6.6.6.6, 203.0.113.9"); res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the proxy's address for the caller is throttled: %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"james","password":"pw"}`, "", "X-Forwarded-For", "6.6.6.6, 198.51.100.7"); res.StatusCode != http.StatusNoContent {
		t.Fatalf("a different caller (a spoofed first entry doesn't matter): %d", res.StatusCode)
	}
}

func TestSecurityHeadersAndHealth(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	res, body := do(t, "GET", r.srv.URL+"/healthz", "", "")
	if res.StatusCode != http.StatusOK || body != "ok\n" {
		t.Fatalf("healthz: %d %q", res.StatusCode, body)
	}
	h := res.Header
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" ||
		!strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") || h.Get("Strict-Transport-Security") != "" {
		t.Fatalf("headers: %v", h)
	}
	if res, _ := do(t, "GET", r.srv.URL+"/healthz", "", "", "X-Forwarded-Proto", "https"); res.Header.Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS once served over HTTPS")
	}
}
