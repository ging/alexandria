// Tests the upstream fetcher: https enforcement, the localhost exception, the allow-list and the redirect limit.
// Fixtures: an httptest upstream serving a JSON-LD context and a redirect loop (T-HUB-027, DP-13).
package fetch_http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// upstream serves a context at /ctx and an endless redirect at /loop.
func upstream(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/ld+json")
		_, _ = w.Write([]byte(`{"@context":{}}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestLocalhostOnlyWhenAllowed(t *testing.T) {
	s := upstream(t)
	if _, _, err := (Fetcher{Timeout: time.Second}).Fetch(s.URL+"/ctx", 100); err == nil {
		t.Fatal("http accepted without the localhost option")
	}
	if _, ct, err := (Fetcher{Timeout: time.Second, AllowInsecureLocalhost: true}).Fetch(s.URL+"/ctx", 100); err != nil || ct != "application/ld+json" {
		t.Fatalf("got %s %v", ct, err)
	}
}

func TestAllowList(t *testing.T) {
	s := upstream(t)
	f := Fetcher{Timeout: time.Second, AllowInsecureLocalhost: true, Allow: []string{"*.w3.org"}}
	if _, _, err := f.Fetch(s.URL+"/ctx", 100); err == nil {
		t.Fatal("host outside the allow-list accepted")
	}
}

func TestRedirectLimit(t *testing.T) {
	s := upstream(t)
	if _, _, err := (Fetcher{Timeout: time.Second, AllowInsecureLocalhost: true}).Fetch(s.URL+"/loop", 100); err == nil {
		t.Fatal("redirect loop accepted")
	}
}
