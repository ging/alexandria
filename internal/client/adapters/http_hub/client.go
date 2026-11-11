// Package http_hub implements the HTTP client of the hub: the namespace listing from /v1 and package manifests and blobs from /pub, all without credentials.
// The only adapter that talks to the hub; it is wired into Sync, never into Edge (R16, REQ-SYS-002, REQ-CLI-003, REQ-CLI-004).
package http_hub

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// Client fetches listings and packages.
type Client struct {
	Base      string
	Namespace string
	HTTP      *http.Client
}

var _ app.HubAPI = (*Client)(nil)

// New builds a client with a bounded timeout; caFile optionally adds a PEM root for a lab hub certificate (DP-14).
func New(base, namespace, caFile string) (*Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if caFile != "" {
		pem, err := os.ReadFile(caFile) // #nosec G304 -- operator-chosen CA file.
		if err != nil {
			return nil, err
		}
		pool, _ := x509.SystemCertPool()
		if pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("http_hub: no certificate in " + caFile)
		}
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &Client{Base: strings.TrimSuffix(base, "/"), Namespace: namespace, HTTP: &http.Client{Timeout: 15 * time.Second, Transport: tr}}, nil
}

// get performs a GET and returns status, headers and body.
func (c *Client) get(path string) (int, http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.Base+path, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return resp.StatusCode, resp.Header, b, err
}

// Listing implements S1: 404 is a missing namespace, 5xx or no answer is unreachable.
func (c *Client) Listing() (app.ListingResponse, error) {
	st, hdr, b, err := c.get("/v1/namespaces/" + c.Namespace + "/versions")
	switch {
	case err != nil:
		return app.ListingResponse{}, err
	case st == http.StatusNotFound:
		return app.ListingResponse{}, app.ErrNamespaceNotFound
	case st != http.StatusOK:
		return app.ListingResponse{}, fmt.Errorf("listing answered %d", st)
	}
	out := app.ListingResponse{Body: b}
	if d, err := http.ParseTime(hdr.Get("Date")); err == nil {
		out.Date, out.HasDate = timex.FromTime(d), true
	}
	return out, nil
}

// Package downloads manifest.json and every listed blob.
func (c *Client) Package(d ni.Digest) (pkgv.Files, error) {
	st, _, m, err := c.get("/pub/packages/" + d.Segment() + "/manifest.json")
	if err != nil {
		return pkgv.Files{}, err
	}
	if st != http.StatusOK {
		return pkgv.Files{}, fmt.Errorf("manifest.json answered %d", st)
	}
	f := pkgv.Files{Manifest: m, Content: map[string][]byte{}}
	cred, err := pkgv.Parse(m)
	if err != nil {
		return f, nil
	}
	for _, e := range cred.Subject.Files {
		st, _, b, err := c.get("/pub/blobs/sha256/" + e.Digest.Hex())
		if err != nil {
			return pkgv.Files{}, err
		}
		if st != http.StatusOK {
			return pkgv.Files{}, fmt.Errorf("blob %s answered %d", e.Path, st)
		}
		f.Content[e.Path] = b
	}
	return f, nil
}
