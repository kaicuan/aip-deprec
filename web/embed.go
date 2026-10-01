package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distEmbedFS embed.FS

// GetFS returns an fs.FS rooted at the "dist" folder
func GetFS() fs.FS {
	sub, err := fs.Sub(distEmbedFS, "dist")
	if err != nil {
		return distEmbedFS
	}
	return sub
}
