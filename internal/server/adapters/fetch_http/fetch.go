// Package fetch_http downloads proxied vocabularies: https only, at most 5 redirects, size and time limits and a host allow-list.
// http://localhost is accepted only when allowed for tests (§3.6, Q22, DP-13, REQ-HUB-027).
package fetch_http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/server/app"
)

// Fetcher implements app.UpstreamFetcher.
type Fetcher struct {
	Timeout                time.Duration
	Allow                  []string
	AllowInsecureLocalhost bool
}

var _ app.UpstreamFetcher = Fetcher{}

// Fetch downloads url, reading at most limit bytes.
func (f Fetcher) Fetch(raw string, limit int64) ([]byte, string, error) {
	if err := f.check(raw); err != nil {
		return nil, "", err
	}
	c := &http.Client{Timeout: f.Timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("more than 5 redirects")
		}
		return f.check(r.URL.String())
	}}
	resp, err := c.Get(raw) // #nosec G107 -- upstream registered by a steward and checked against the allow-list.
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream answered %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return b, resp.Header.Get("Content-Type"), err
}

// check accepts https URLs whose host matches the allow-list (empty list: any host).
func (f Fetcher) check(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && f.AllowInsecureLocalhost && did.IsLocalhost(u.Host)) {
		return fmt.Errorf("%s is not https", raw)
	}
	if len(f.Allow) == 0 {
		return nil
	}
	for _, p := range f.Allow {
		if ok, _ := path.Match(p, u.Hostname()); ok {
			return nil
		}
	}
	return fmt.Errorf("host %s is not allowed", u.Hostname())
}
