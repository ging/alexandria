// HTTP contract of /pub (§3.10): immutable routes with SHA-256 ETags, the DP-12 head redirect, vocabulary URLs and /pub/_vhub.
// Fixtures: the testkit HTTP hub with the F6 releases and a proxied vocabulary (T-HUB-011, 016, 026, 027, 030, T-SYS-005).
package http_pub_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// hub publishes the F6 releases and a proxied vocabulary.
func hub(t *testing.T) *testkit.HTTPServer {
	t.Helper()
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.Setup(t)
	h.PublishF6(t)
	if _, err := h.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "dcat-ap-context", Kind: "proxy", MediaType: "application/ld+json", Upstream: "https://semiceu.github.io/ctx.jsonld"}); err != nil {
		t.Fatal(err)
	}
	return h
}

// immutablePaths lists the manifest, blobs and versioned artefacts of the published packages.
func immutablePaths(t *testing.T, h *testkit.HTTPServer) []string {
	t.Helper()
	_, _, b := h.Do(t, "GET", "/v1/namespaces/demo/versions", "", nil, nil)
	var l vc.Credential[lifecycle.ListingSubject]
	_ = json.Unmarshal(b, &l)
	out := []string{"/pub/_vhub/v1/context.jsonld", "/pub/_vhub/v1/vocab.ttl", "/pub/demo/vocab/dcat-ap-context/r/1"}
	for _, r := range l.Subject.Versions {
		out = append(out, "/pub/packages/"+r.Manifest.Segment()+"/manifest.json")
		_, _, mb := h.Do(t, "GET", "/pub/packages/"+r.Manifest.Segment()+"/manifest.json", "", nil, nil)
		c, _ := pkgv.Parse(mb)
		for _, f := range c.Subject.Files {
			out = append(out, "/pub/blobs/sha256/"+f.Digest.Hex(), "/pub/demo/"+r.Version.String()+"/"+f.Path)
		}
	}
	return out
}

func TestT_HUB_011_ImmutableRoutesHaveHashETags(t *testing.T) {
	h := hub(t)
	for _, p := range immutablePaths(t, h) {
		st, hdr, b := h.Do(t, "GET", p, "", nil, nil)
		if st != 200 || hdr.Get("ETag") != `"`+ni.HexSum(b)+`"` || !strings.Contains(hdr.Get("Cache-Control"), "immutable") {
			t.Errorf("%s: %d etag %s cache %s", p, st, hdr.Get("ETag"), hdr.Get("Cache-Control"))
		}
	}
}

func TestT_SYS_005_BytesNeverChange(t *testing.T) {
	h := hub(t)
	paths := immutablePaths(t, h)
	before := map[string][]byte{}
	for _, p := range paths {
		_, _, before[p] = h.Do(t, "GET", p, "", nil, nil)
	}
	h.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = h.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
	for _, p := range paths {
		if _, _, b := h.Do(t, "GET", p, "", nil, nil); !bytes.Equal(b, before[p]) {
			t.Errorf("%s changed", p)
		}
	}
}

func TestT_HUB_016_HeadRedirectsToManifestWithoutContext(t *testing.T) {
	h := hub(t)
	h.Clock.Set("2026-10-10T12:00:00Z")
	for _, accept := range []string{"application/ld+json", "text/turtle", "*/*"} {
		st, hdr, _ := h.Do(t, "GET", "/pub/demo", "", nil, map[string]string{"Accept": accept})
		if st != 303 || !strings.HasPrefix(hdr.Get("Location"), "/pub/packages/") || hdr.Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d %s", accept, st, hdr.Get("Location"))
		}
	}
}

func TestT_HUB_026_StableVocabularyURLRedirects(t *testing.T) {
	h := hub(t)
	st, hdr, _ := h.Do(t, "GET", "/pub/demo/vocab/dcat-ap-context", "", nil, nil)
	if st != 303 || hdr.Get("Location") != "/pub/demo/vocab/dcat-ap-context/r/1" || hdr.Get("Cache-Control") != "no-cache" {
		t.Fatalf("%d %s", st, hdr.Get("Location"))
	}
}

func TestT_HUB_026_ProxyRevisionHasLinkVia(t *testing.T) {
	h := hub(t)
	_, hdr, _ := h.Do(t, "GET", "/pub/demo/vocab/dcat-ap-context/r/1", "", nil, nil)
	if hdr.Get("Link") != `<https://semiceu.github.io/ctx.jsonld>; rel="via"` || hdr.Get("Content-Type") != "application/ld+json" {
		t.Fatalf("link %q type %q", hdr.Get("Link"), hdr.Get("Content-Type"))
	}
}

func TestT_HUB_027_PubReadsNeverFetch(t *testing.T) {
	h := hub(t)
	calls := h.Fetcher.Calls
	for i := 0; i < 5; i++ {
		_, _, _ = h.Do(t, "GET", "/pub/demo/vocab/dcat-ap-context/r/1", "", nil, nil)
		_, _, _ = h.Do(t, "GET", "/pub/demo/vocab/dcat-ap-context", "", nil, nil)
	}
	if h.Fetcher.Calls != calls {
		t.Fatalf("%d upstream calls during /pub reads", h.Fetcher.Calls-calls)
	}
}

func TestT_HUB_030_ContextIsAppendixA5ByteForByte(t *testing.T) {
	h := hub(t)
	_, _, b := h.Do(t, "GET", "/pub/_vhub/v1/context.jsonld", "", nil, nil)
	if !bytes.Equal(b, schemas.Context()) {
		t.Fatal("served context differs from Appendix A.5")
	}
	_, _, ttl := h.Do(t, "GET", "/pub/_vhub/v1/vocab.ttl", "", nil, nil)
	if !bytes.Equal(ttl, http_pub.VocabTTL()) || !strings.Contains(string(ttl), "avh:VersionListing") {
		t.Fatal("vocab.ttl not served")
	}
}
