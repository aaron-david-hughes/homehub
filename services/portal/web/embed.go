// Package web embeds the portal's static assets (CSS, JS).
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var content embed.FS

// Static returns the static assets rooted at the static directory.
func Static() fs.FS {
	sub, err := fs.Sub(content, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
