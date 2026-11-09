// HTTP client of the hub: GET /v1/versions with the member bearer token, and package files and blobs from /pub.
// The only adapter that talks to the hub; it is wired into Sync, never into Edge (REQ-SYS-002, REQ-CLI-003, REQ-CLI-027, DP-3).
package http_hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// Client fetches listings and packages.
type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

var _ app.HubAPI = (*Client)(nil)

// New builds a client with a bounded timeout.
func New(base, token string) *Client {
	return &Client{Base: strings.TrimSuffix(base, "/"), Token: token, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// get performs a GET and returns status, headers and body.
func (c *Client) get(path string, auth bool) (int, http.Header, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.Base+path, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("%w: %v", app.ErrUnreachable, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return resp.StatusCode, resp.Header, b, err
}

// Listing implements S1.
func (c *Client) Listing() (app.ListingResponse, error) {
	st, hdr, b, err := c.get("/v1/versions", true)
	switch {
	case err != nil:
		return app.ListingResponse{}, err
	case st == http.StatusUnauthorized || st == http.StatusForbidden:
		return app.ListingResponse{}, app.ErrUnauthorized
	case st != http.StatusOK:
		return app.ListingResponse{}, fmt.Errorf("%w: GET /v1/versions answered %d", app.ErrUnreachable, st)
	}
	out := app.ListingResponse{Body: b}
	if d, err := http.ParseTime(hdr.Get("Date")); err == nil {
		out.Date, out.HasDate = timex.FromTime(d), true
	}
	return out, nil
}

// Package downloads manifest.json, manifest.sig, manifest.receipt, space-lock.json and every listed blob.
func (c *Client) Package(d ni.Digest) (pkgv.Files, error) {
	var f pkgv.Files
	for name, dst := range map[string]*[]byte{"manifest.json": &f.Manifest, "manifest.sig": &f.Sig, "manifest.receipt": &f.Receipt, "space-lock.json": &f.SpaceLock} {
		st, _, b, err := c.get("/pub/packages/"+d.Segment()+"/"+name, false)
		if err != nil {
			return pkgv.Files{}, err
		}
		if st != http.StatusOK {
			return pkgv.Files{}, fmt.Errorf("%w: %s answered %d", app.ErrUnreachable, name, st)
		}
		*dst = b
	}
	var m pkgv.Manifest
	if err := json.Unmarshal(f.Manifest, &m); err != nil {
		return f, nil
	}
	f.Content = map[string][]byte{}
	for _, e := range m.Files {
		st, _, b, err := c.get("/pub/blobs/sha256/"+e.Digest.Hex(), false)
		if err != nil {
			return pkgv.Files{}, err
		}
		if st != http.StatusOK {
			return pkgv.Files{}, fmt.Errorf("%w: blob %s answered %d", app.ErrUnreachable, e.Path, st)
		}
		f.Content[e.Path] = b
	}
	return f, nil
}
