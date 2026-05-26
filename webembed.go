package cternal

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var distEmbed embed.FS

// FrontendFS returns the embedded frontend static files.
func FrontendFS() fs.FS {
	sub, err := fs.Sub(distEmbed, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
