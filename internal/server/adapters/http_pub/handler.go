// Package http_pub is the driving adapter of the public /pub API: head and vocabulary redirects, immutable artefacts, manifests, blobs and /pub/_vhub.
// Paths are routed by shape in one handler because the URL forms of §3.10 overlap as mux patterns (REQ-SYS-005, REQ-HUB-011, REQ-HUB-016, REQ-HUB-026, REQ-HUB-030).
package http_pub

import (
	_ "embed"
	"net/http"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

//go:embed vocab.ttl
var vocabTTL []byte

// VocabTTL returns the embedded vocabulary description.
func VocabTTL() []byte { return vocabTTL }

// Handler serves /pub.
type Handler struct {
	Pub      *app.Publication
	Problems httpx.Problems
}

// Register mounts /pub on mux.
func (h Handler) Register(mux *http.ServeMux) { mux.HandleFunc("GET /pub/", h.serve) }

// serve routes /pub paths by their shape.
func (h Handler) serve(w http.ResponseWriter, r *http.Request) {
	seg := strings.Split(strings.TrimPrefix(r.URL.Path, "/pub/"), "/")
	n := len(seg)
	switch {
	case n == 3 && seg[0] == domain.ReservedSlug && seg[1] == "v1" && seg[2] == "context.jsonld":
		h.immutable(w, schemas.Context(), "application/ld+json", nil)
	case n == 3 && seg[0] == domain.ReservedSlug && seg[1] == "v1" && seg[2] == "vocab.ttl":
		h.immutable(w, vocabTTL, "text/turtle", nil)
	case n == 3 && seg[0] == "packages" && seg[2] == "manifest.json":
		b, err := h.Pub.Manifest(seg[1])
		h.immutable(w, b, "application/ld+json", err)
	case n == 3 && seg[0] == "blobs" && seg[1] == "sha256":
		b, err := h.Pub.Blob(seg[2])
		h.immutable(w, b, "application/octet-stream", err)
	case n == 1 && seg[0] != "":
		loc, err := h.Pub.Head(seg[0])
		h.redirect(w, loc, err)
	case n == 3 && seg[1] == "vocab":
		loc, err := h.Pub.VocabLatest(seg[0], seg[2])
		h.redirect(w, loc, err)
	case n == 5 && seg[1] == "vocab" && seg[3] == "r":
		h.revision(w, seg[0], seg[2], seg[4])
	case n >= 3:
		a, err := h.Pub.VersionFile(seg[0], seg[1], strings.Join(seg[2:], "/"))
		h.immutable(w, a.Bytes, a.MediaType, err)
	default:
		h.Problems.Error(w, codes.New(codes.APINotFound, "%s not found", r.URL.Path))
	}
}

// revision serves the bytes of a vocabulary revision with Link via for proxies.
func (h Handler) revision(w http.ResponseWriter, ns, name, nstr string) {
	n, err := strconv.Atoi(nstr)
	if err != nil {
		h.Problems.Error(w, codes.New(codes.APINotFound, "revision %s not found", nstr))
		return
	}
	v, b, err := h.Pub.VocabRevision(ns, name, n)
	if err == nil && v.Kind == domain.Proxy {
		w.Header().Set("Link", "<"+v.Upstream+`>; rel="via"`)
	}
	h.immutable(w, b, v.MediaType, err)
}

// redirect answers 303 with Cache-Control no-cache.
func (h Handler) redirect(w http.ResponseWriter, loc string, err error) {
	if err != nil {
		h.Problems.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Location", loc)
	w.WriteHeader(http.StatusSeeOther)
}

// immutable writes bytes with immutable caching and a SHA-256 hex ETag.
func (h Handler) immutable(w http.ResponseWriter, b []byte, mediaType string, err error) {
	if err != nil {
		h.Problems.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+ni.HexSum(b)+`"`)
	w.Header().Set("Content-Type", mediaType)
	_, _ = w.Write(b) // #nosec G705 -- immutable bytes served with their registered media type.
}
