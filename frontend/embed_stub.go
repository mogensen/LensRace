//go:build !embed_frontend

package frontend

import "embed"

// DistFS is empty unless the binary was built with `-tags embed_frontend`
// (see embed.go and the Makefile's `build` target). The server checks for
// this and simply skips static file serving when it's empty, which is the
// normal case for local development (Vite serves the frontend instead).
var DistFS embed.FS

// DistDir mirrors the constant in embed.go so callers don't need a build
// tag of their own to reference it.
const DistDir = "dist"
