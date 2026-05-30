package api

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// StaticFS is set by the main package via go:embed from the module root.
var StaticFS fs.FS

// StaticHandler returns an http.Handler that serves the embedded frontend under basePath.
// It strips basePath from the URL, serves static assets as-is, and falls back to index.html
// for unknown paths (SPA routing). The basePath is injected into index.html as window.__BASE_PATH__.
func StaticHandler(basePath string) http.Handler {
	if StaticFS == nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(StaticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, basePath)
		if path == "" {
			path = "/"
		}

		// Serve static assets that exist in the embedded FS.
		if path != "/" && path != "/index.html" {
			trimmed := strings.TrimPrefix(path, "/")
			if _, err := fs.Stat(StaticFS, trimmed); err == nil {
				r2 := r.Clone(r.Context())
				r2.URL.Path = path
				fileServer.ServeHTTP(w, r2)
				return
			}
		}

		// SPA fallback: serve index.html with injected base path.
		serveIndex(w, basePath)
	})
}

func serveIndex(w http.ResponseWriter, basePath string) {
	data, err := fs.ReadFile(StaticFS, "index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	script := fmt.Sprintf(`<script>window.__BASE_PATH__=%q</script>`, basePath)
	data = bytes.ReplaceAll(data, []byte("</head>"), []byte(script+"</head>"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
