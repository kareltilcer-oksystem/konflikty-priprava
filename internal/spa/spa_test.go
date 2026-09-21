package spa

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// get exercises the handler the binary actually serves.
func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// Without a built frontend the binary must still run: the API is unaffected and
// the SPA listener explains itself rather than 404ing silently.
func TestStubWhenNoFrontendIsBuilt(t *testing.T) {
	if Available() {
		t.Skip("a frontend is embedded in this build; the stub path is not reachable")
	}
	rec := get(t, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("stub status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "npm --prefix web run build") {
		t.Errorf("the stub should say how to fix it: %s", rec.Body.String())
	}
}

func TestDeepLinksAndMissingAssets(t *testing.T) {
	if !Available() {
		t.Skip("no frontend embedded; run npm --prefix web run build first")
	}

	// A client route must serve index.html so /porada/2026-w38 can be opened
	// directly or reloaded.
	for _, path := range []string{"/", "/porady", "/porada/2026-w38", "/problem/42"} {
		rec := get(t, path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET %s served %q, want HTML", path, ct)
		}
	}

	// A missing asset must 404 rather than fall through to index.html: an HTML
	// body under a .js URL turns a deploy mistake into a baffling syntax error.
	for _, path := range []string{"/assets/missing.js", "/nope.css", "/favicon-typo.ico"} {
		rec := get(t, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<!doctype html") {
			t.Errorf("GET %s returned the SPA shell instead of a 404", path)
		}
	}

	// index.html must not be cached, or a deploy would not reach open tabs.
	if cc := get(t, "/").Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", cc)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	if !Available() {
		t.Skip("no frontend embedded")
	}
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/porada/2026-w38", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST to a client route = %d, want 405", rec.Code)
	}
}

func TestContentType(t *testing.T) {
	cases := map[string]string{
		"index.html":    "text/html; charset=utf-8",
		"assets/a.js":   "text/javascript; charset=utf-8",
		"assets/a.css":  "text/css; charset=utf-8",
		"f.woff2":       "font/woff2",
		"logo.svg":      "image/svg+xml",
		"x.png":         "image/png",
		"unknown.thing": "application/octet-stream",
	}
	for name, want := range cases {
		if got := contentType(name); got != want {
			t.Errorf("contentType(%q) = %q, want %q", name, got, want)
		}
	}
}
