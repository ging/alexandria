//go:build !gui

// Package webui omits the console in the default build: no assets are embedded, so binaries need neither Node nor a prior gui build.
// Mount becomes a no-op and the listeners expose only their APIs.
package webui

import "io/fs"

// assets is nil when the console is not compiled in.
var assets fs.FS
