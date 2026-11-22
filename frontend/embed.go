// Package frontend is the desktop app's interface, built into the binary.
package frontend

import "embed"

// Assets are the interface as it was built into dist, which the binary
// serves to its window.
//
//go:embed all:dist
var Assets embed.FS
