// Package frontend embeds the built Vite app (frontend/dist) into the Go binary.
// Run `npm run build` in this directory before `go build`.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the contents of the dist directory.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
