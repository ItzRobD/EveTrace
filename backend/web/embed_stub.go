//go:build !embed

package web

import "embed"

// EmbeddedFiles is empty in dev builds (no -tags embed).
// The server falls back to API-only mode and CORS allows the dev frontend at :4200.
var EmbeddedFiles embed.FS
