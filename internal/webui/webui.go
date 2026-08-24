// Package webui carries the compiled single-page app inside the binary, so that
// one process serves both the API and the UI.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS is the built SPA. Built reports whether a frontend build was embedded: a
// bare `go build` without `pnpm build` produces a binary with an empty UI, and
// the HTTP surface says so plainly rather than serving a blank page.
func FS() (files fs.FS, built bool) {
	files, err := fs.Sub(embedded, "dist/spa")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return files, false
	}
	return files, true
}
