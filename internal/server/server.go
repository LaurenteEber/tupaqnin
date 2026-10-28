// Package server implements HTTP, streaming and access for tupaqnin
// (specification §6). In this base skeleton it only serves /healthz
// and the embedded minimal page; streaming and invitation access
// arrive in slice 0 and the delivery slice.
package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

// securityHeaders sets the security headers required by specification
// §6 (CSP with no inline scripts, no MIME sniffing, no framing).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// New returns the HTTP handler for the application.
func New() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	return securityHeaders(mux)
}
