// Package http_ui is the driving adapter that serves the embedded console on the admin listener under /ui/.
// It is mounted only beside the admin API, never on DSP or management listeners, and exposes the client workspace.
package http_ui

import (
	"net/http"

	"github.com/ging/alexandria/internal/webui"
)

// Handler mounts the client workspace of the console.
type Handler struct{}

// Register mounts /ui/ on mux and reports whether the console is compiled into this binary.
func (Handler) Register(mux *http.ServeMux) bool {
	return webui.Mount(mux, webui.Config{Workspace: "client"})
}
