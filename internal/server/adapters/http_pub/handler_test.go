// HTTP contract tests of /pub: immutability with SHA-256 ETags and the DP-15 head redirect.
// Realise T-HUB-011 and T-HUB-016 (§3.7).
package http_pub_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/testkit"
)

// immutablePaths lists every immutable route of the published F6 packages.
func immutablePaths(t *testing.T, h *testkit.HTTPServer) []string {
	t.Helper()
	_, _, b := h.Do(t, "GET", "/v1/versions", testkit.MemberToken, nil, nil)
	var env sign.Envelope[lifecycle.Listing]
	_ = json.Unmarshal(b, &env)
	var out []string
	for _, r := range env.Signed.Versions {
		seg := r.Manifest.Segment()
		for _, f := range []string{"manifest.json", "manifest.sig", "manifest.receipt", "space-lock.json"} {
			out = append(out, "/pub/packages/"+seg+"/"+f)
		}
		_, _, mb := h.Do(t, "GET", "/pub/packages/"+seg+"/manifest.json", "", nil, nil)
		var m pkgv.Manifest
		_ = json.Unmarshal(mb, &m)
		for _, f := range m.Files {
			out = append(out, "/pub/blobs/sha256/"+f.Digest.Hex(), "/pub/demo/"+r.Version.String()+"/"+f.Path)
		}
		out = append(out, "/pub/demo/"+r.Version.String()+"/manifest.json")
	}
	return out
}

func TestT_HUB_011_ImmutableBytesAndETag(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	paths := immutablePaths(t, h)
	first := map[string]string{}
	for round := 0; round < 2; round++ {
		if round == 1 {
			h.Publish(t, "2026-10-10T10:00:00Z", testkit.Draft("2.1.0", "odrl:use"), "")
		}
		for _, p := range paths {
			st, hdr, b := h.Do(t, "GET", p, "", nil, nil)
			sum := sha256.Sum256(b)
			etag := `"` + hex.EncodeToString(sum[:]) + `"`
			if st != http.StatusOK || hdr.Get("ETag") != etag || hdr.Get("Cache-Control") != "public, max-age=31536000, immutable" {
				t.Fatalf("%s: %d etag %s cache %s", p, st, hdr.Get("ETag"), hdr.Get("Cache-Control"))
			}
			if round == 0 {
				first[p] = etag
			} else if first[p] != etag {
				t.Fatalf("%s changed bytes", p)
			}
		}
	}
}

func TestT_HUB_016_RedirectForEveryMediaType(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	h.Clock.Set("2026-10-10T12:00:00Z")
	for _, accept := range []string{"application/json", "application/ld+json", "text/turtle", "application/schema+json", "*/*", ""} {
		for _, p := range []string{"/pub/demo", "/pub/spaces/demo"} {
			st, hdr, _ := h.Do(t, "GET", p, "", nil, map[string]string{"Accept": accept})
			if st != http.StatusSeeOther || hdr.Get("Location") != "/pub/demo/2.0.0/manifest.json" || hdr.Get("Cache-Control") != "no-cache" {
				t.Errorf("%s Accept %q: %d %s", p, accept, st, hdr.Get("Location"))
			}
		}
	}
}

func TestVersionedFileServesManifestMediaType(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	if st, hdr, _ := h.Do(t, "GET", "/pub/demo/1.0.0/schemas/policy.schema.json", "", nil, nil); st != http.StatusOK || hdr.Get("Content-Type") != "application/schema+json" {
		t.Fatalf("got %d %s", st, hdr.Get("Content-Type"))
	}
}

func TestUnreferencedBlobIsNotServed(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	if st, _, _ := h.Do(t, "GET", "/pub/blobs/sha256/"+hex.EncodeToString(make([]byte, 32)), "", nil, nil); st != http.StatusNotFound {
		t.Fatalf("got %d", st)
	}
}
