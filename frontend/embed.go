//go:build embed_frontend

// Package frontend embeds the built Vue SPA (frontend/dist) into the Go
// binary so the backend can serve it directly, for a single deployable
// artifact. Requires `pnpm build` to have produced frontend/dist *before*
// `go build -tags embed_frontend` runs — see the Makefile's `build` target,
// which does both in order.
package frontend

import "embed"

//go:embed all:dist
var DistFS embed.FS

// DistDir is the subdirectory within DistFS that holds the built assets.
const DistDir = "dist"
