//go:build gui

// Package webui embeds the production build of gui/ (written to dist/ by "pnpm build:embed") into binaries built with -tags gui.
// The dist directory is a build artefact and is not committed; "make build-gui" produces it before compiling.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// assets is the console build rooted at dist/.
var assets = sub(dist)

// sub roots the embedded tree at dist/.
func sub(d embed.FS) fs.FS {
	s, err := fs.Sub(d, "dist")
	if err != nil {
		panic(err)
	}
	return s
}
