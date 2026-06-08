// Package web embeds the built React SPA (web/dist) into the Go binary.
//
// The embed must live in the package whose directory contains dist/, because
// Go's //go:embed cannot reference parent directories ("..") — so the embed
// directive lives here at the web/ root and internal/api consumes the fs.FS.
//
// web/dist/index.html is a placeholder until the Vite build (Wave F) produces
// the real bundle; CI builds the React app before `go build` so the embedded
// dist is the real SPA in shipped images.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded web/dist directory rooted at its top level
// (so "index.html" resolves directly), or panics if the embed is malformed.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Unreachable: "dist" is embedded at build time.
		panic("web: embedded dist subtree missing: " + err.Error())
	}
	return sub
}
