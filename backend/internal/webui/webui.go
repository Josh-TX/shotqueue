// Package webui exposes the built Vue frontend (the frontend build writes dist/
// here). It is only embedded when built with -tags embed_frontend.
package webui

import "io/fs"

// FS returns the embedded frontend build rooted at dist/.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Embedded reports whether a frontend build is embedded.
func Embedded() bool {
	_, err := fs.Stat(distFS, "dist/index.html")
	return err == nil
}
