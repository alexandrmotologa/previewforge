package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/previewforge/web"
)

// FileServer returns an http.Handler that serves embedded frontend files with SPA fallback.
func FileServer() http.Handler {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<h1>PreviewForge Web Studio</h1><p>Frontend assets not embedded yet.</p>"))
		})
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if file exists in dist
		f, err := distFS.Open(path)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA HTML5 fallback to index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
