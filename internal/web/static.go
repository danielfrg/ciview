// Package web embeds and serves the verified frontend build.
package web

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// Build recipes replace dist with verified Vite output before compilation.
//
//go:embed all:dist
var files embed.FS

func Assets() fs.FS { assets, _ := fs.Sub(files, "dist"); return assets }

// NewHandler serves only actual files. There is no directory listing, hidden
// file access, or app-shell fallback for unknown API or asset URLs.
func NewHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) || strings.HasPrefix(name, "api/") || name == "api" {
			http.NotFound(w, r)
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			if r.URL.Path == "/" {
				http.Error(w, "Frontend assets unavailable; run just build", http.StatusServiceUnavailable)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
