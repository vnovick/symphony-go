//go:build dev

package server

import (
	"net/http"
)

// spaFS returns a filesystem serving web/dist from disk (dev mode).
// Run with: go run -tags dev ./cmd/symphony
func spaFS() http.FileSystem {
	return http.Dir("internal/server/web/dist")
}
