// Package templates embeds the portal's HTML templates.
package templates

import "embed"

//go:embed *.html
var FS embed.FS
