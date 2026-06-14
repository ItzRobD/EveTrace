//go:build embed

package web

import "embed"

//go:embed all:dist
var EmbeddedFiles embed.FS
