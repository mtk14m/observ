// Package ui serves the web UI embedded in the obsrv binary.
//
// The built UI (web/dist) is copied into ./dist by `make web-build` before
// `go build`, then embedded.
package ui

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Dist returns the embedded UI files.
func Dist() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // the directory is embedded at compile time
	}
	return sub
}

const placeholder = `<!doctype html><title>obsrv</title>
<p>The obsrv UI is not built into this binary. Run <code>make web-build</code>, then rebuild.
The API is available under <a href="/api/v1/services">/api/v1</a>.</p>`

// NewHandler serves files from dist. Paths without a file extension are
// client-side routes and get index.html; missing assets get a 404.
func NewHandler(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && path.Ext(name) != "" {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		index, err := dist.Open("index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, placeholder)
			return
		}
		defer func() { _ = index.Close() }()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.Copy(w, index)
	})
}
