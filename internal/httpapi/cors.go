package httpapi

import (
	"net/http"
	"strings"
)

// CORS wraps the API listener so named origins can call it cross-origin with
// credentials.
//
// It is applied ONLY to the standalone API listener. The SPA listener serves
// /api from the same origin as the page, so it needs no CORS headers at all —
// and an empty CORS_ORIGINS, the normal case, returns the handler untouched so
// no header is emitted anywhere.
//
// "*" is rejected at start-up rather than here: browsers refuse the wildcard on
// credentialed requests, so accepting it would produce a configuration that can
// never work.
func CORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[strings.ToLower(o)] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[strings.ToLower(origin)] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			// The response varies by Origin, so a shared cache must not reuse
			// one origin's response for another.
			h.Add("Vary", "Origin")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				// Answered here and not passed on: ServeMux has no OPTIONS
				// route and would reply 405, which kills the preflight.
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
