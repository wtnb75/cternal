package api

import (
	"io/fs"
	"net/http"
)

// StaticFS is set by the main package via go:embed from the module root.
// This indirection is needed because go:embed cannot reference paths above the
// package directory.
var StaticFS fs.FS

// StaticHandler returns an http.Handler that serves the embedded frontend.
func StaticHandler() http.Handler {
	if StaticFS == nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(StaticFS))
}
