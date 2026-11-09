// Driving adapter for the public /pub API: head redirect, versioned artefacts, package files and blobs by hash.
// Versioned and hash-addressed routes are immutable with ETag = SHA-256 of the bytes (REQ-SYS-005, REQ-HUB-011, REQ-HUB-016, DP-15).
package http_pub

import (
	"net/http"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/app"
)

// Handler serves /pub.
type Handler struct{ Pub *app.Publication }

// Register mounts /pub on mux.
func (h Handler) Register(mux *http.ServeMux) { mux.HandleFunc("GET /pub/", h.serve) }

// serve routes /pub paths by their shape.
func (h Handler) serve(w http.ResponseWriter, r *http.Request) {
	seg := strings.Split(strings.TrimPrefix(r.URL.Path, "/pub/"), "/")
	switch {
	case len(seg) == 1 && seg[0] != "":
		h.redirect(w, seg[0])
	case len(seg) == 2 && seg[0] == "spaces":
		h.redirect(w, seg[1])
	case len(seg) == 3 && seg[0] == "packages":
		b, err := h.Pub.PackageFile(seg[1], seg[2])
		immutable(w, b, app.JSONType, err)
	case len(seg) == 3 && seg[0] == "blobs" && seg[1] == "sha256":
		b, err := h.Pub.Blob(seg[2])
		immutable(w, b, "application/octet-stream", err)
	case len(seg) >= 3:
		b, mt, err := h.Pub.VersionedFile(seg[0], seg[1], strings.Join(seg[2:], "/"))
		immutable(w, b, mt, err)
	default:
		httpx.WriteError(w, codes.New(codes.ApiNotFound, "%s not found", r.URL.Path))
	}
}

// redirect answers 303 to the manifest of the current head for any Accept (DP-15).
func (h Handler) redirect(w http.ResponseWriter, slug string) {
	v, err := h.Pub.Head(slug)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Location", "/pub/"+slug+"/"+v.String()+"/manifest.json")
	w.WriteHeader(http.StatusSeeOther)
}

// immutable writes bytes with immutable caching and a SHA-256 ETag.
func immutable(w http.ResponseWriter, b []byte, mediaType string, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+ni.HexSum(b)+`"`)
	w.Header().Set("Content-Type", mediaType)
	_, _ = w.Write(b)
}
