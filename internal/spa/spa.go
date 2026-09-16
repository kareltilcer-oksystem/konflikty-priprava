// Package spa serves the built React frontend from inside the binary.
package spa

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// The all: prefix is required, not decorative. A plain `embed dist` skips files
// whose names begin with "." or "_", which would (a) miss the tracked
// dist/.gitkeep and fail the build on a fresh clone with "no matching files
// found", and (b) silently drop Vite chunks named _something.js.
//
//go:embed all:dist
var embedded embed.FS

// assets is the embedded tree rooted at dist.
func assets() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// Only reachable if the embed directive itself is wrong.
		panic("spa: embedded dist is unreadable: " + err.Error())
	}
	return sub
}

// Available reports whether a real frontend was built into this binary.
//
// A fresh clone embeds only the placeholder, so the Go build must still
// succeed: a missing frontend is a deployment mistake to report at start-up,
// not a compile error that blocks working on the backend.
func Available() bool {
	_, err := fs.Stat(assets(), "index.html")
	return err == nil
}

// Handler serves the SPA, or an explanatory stub when none was built.
func Handler() http.Handler {
	if !Available() {
		return http.HandlerFunc(stub)
	}
	return http.HandlerFunc(serve)
}

func serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fsys := assets()
	clean := path.Clean("/" + r.URL.Path)
	name := strings.TrimPrefix(clean, "/")

	if name != "" {
		if body, err := fs.ReadFile(fsys, name); err == nil {
			// Hashed bundles never change under their own name; index.html does.
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.Header().Set("Content-Type", contentType(name))
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
			return
		}
		// A missing asset must 404 rather than fall through to index.html: an
		// HTML body under a .js URL turns a deploy mistake into a baffling
		// syntax error in the console.
		if path.Ext(clean) != "" || strings.HasPrefix(clean, "/assets/") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}

	// Any other path is a client route — /porada/2026-w38 must deep-link.
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(index))
}

// contentType maps an extension to its media type.
//
// This table is explicit rather than mime.TypeByExtension because that function
// seeds itself from the Windows registry, where a stray association can make
// the server hand a .js bundle out as text/plain and the browser refuse it.
func contentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".map":
		return "application/json; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// stubPage is shown when the binary carries no frontend. It is in Czech
// because anyone who reaches it is looking at the running app.
const stubPage = `<!doctype html>
<html lang="cs"><head><meta charset="utf-8"><title>Velké konflikty — příprava</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:34rem;color:#15181c;line-height:1.6}
code{background:#f1f3f5;padding:.15rem .35rem;border-radius:4px}</style></head>
<body><h1>Frontend není sestavený</h1>
<p>Tento binární soubor neobsahuje sestavenou aplikaci. API na <code>/api</code> funguje normálně.</p>
<p>Sestavte frontend a přeložte znovu:</p>
<pre><code>npm --prefix web ci
npm --prefix web run build
go build -o konflikty-priprava.exe ./cmd/server</code></pre>
</body></html>`

func stub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(stubPage))
}
