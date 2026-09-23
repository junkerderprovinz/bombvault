// Package web embeds the built React SPA (web/dist) into the Go binary. It sits
// next to dist/ because //go:embed cannot reach parent directories.
//
// The only tracked file under dist/ is an empty .gitkeep: `all:dist` fails to
// compile without the directory, and `all:` accepts it empty. The bundle comes
// from the Dockerfile's first stage or from `npm --prefix web run build`;
// without one the server answers 500 "SPA index not found". The Go lint jobs
// build no frontend, so they compile the same empty case a fresh clone has.
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
