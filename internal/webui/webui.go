// Package webui serves the Alexandria console (gui/) as a single-page application under /ui/ on the listener that mounts it.
// Assets are compiled in only with the "gui" build tag; without it Enabled reports false and Mount registers nothing.
package webui

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Prefix is the URL subtree of the console.
const Prefix = "/ui/"

// Config is the runtime configuration injected into the page as window.__ALEXANDRIA__.
type Config struct {
	Workspace string `json:"workspace"`
	Issuer    string `json:"issuer,omitempty"`
	Audience  string `json:"audience,omitempty"`
}

// Enabled reports whether the console assets were compiled into this binary.
func Enabled() bool { return assets != nil }

// Mount registers the console on mux and reports whether it did.
func Mount(mux *http.ServeMux, cfg Config) bool {
	if !Enabled() {
		return false
	}
	mux.Handle("GET "+Prefix, New(assets, cfg))
	mux.Handle("GET "+strings.TrimSuffix(Prefix, "/"), http.RedirectHandler(Prefix, http.StatusMovedPermanently))
	return true
}

// New returns the handler that serves static files from dist and the configured index.html for every application route.
func New(dist fs.FS, cfg Config) http.Handler {
	index := inject(dist, cfg)
	files := http.StripPrefix(Prefix, http.FileServerFS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), strings.TrimSuffix(Prefix, "/"))
		name = strings.TrimPrefix(name, "/")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if info, err := fs.Stat(dist, name); name != "" && name != "index.html" && err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		// A missing file with an extension is a broken asset reference, never an application route.
		if index == nil || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index) // #nosec G705 -- embedded page plus HTML-escaped JSON configuration.
	})
}

// inject returns index.html with the configuration script placed first in <head>, or nil when the page is missing.
func inject(dist fs.FS, cfg Config) []byte {
	page, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil
	}
	// json.Marshal escapes <, > and &, so the value cannot close the script element.
	b, _ := json.Marshal(cfg)
	script := append(append([]byte("<head><script>window.__ALEXANDRIA__="), b...), []byte(";</script>")...)
	return bytes.Replace(page, []byte("<head>"), script, 1)
}
