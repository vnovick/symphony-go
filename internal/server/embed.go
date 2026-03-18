//go:build !dev

package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist
var webDistFS embed.FS

// spaFS returns a filesystem rooted at web/dist for SPA serving.
func spaFS() http.FileSystem {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		panic("embed: failed to sub web/dist: " + err.Error())
	}
	return http.FS(sub)
}
