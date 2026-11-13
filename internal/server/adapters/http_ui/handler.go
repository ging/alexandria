// Package http_ui is the driving adapter that serves the embedded console on the hub listener under /ui/.
// It holds no logic: it tells the page it runs beside /v1 and /pub and which issuer and audience to offer for sign-in; laboratory tooling (DP-14).
package http_ui

import (
	"net/http"

	"github.com/ging/alexandria/internal/webui"
)

// Handler mounts the hub workspace of the console.
type Handler struct {
	Issuer   string
	Audience string
}

// Register mounts /ui/ on mux and reports whether the console is compiled into this binary.
func (h Handler) Register(mux *http.ServeMux) bool {
	return webui.Mount(mux, webui.Config{Workspace: "hub", Issuer: h.Issuer, Audience: h.Audience})
}
