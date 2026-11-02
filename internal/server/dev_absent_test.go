//go:build !devroutes

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func TestB4_DevStreamRouteIsAbsentWithoutTheTag(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/dev/stream-test", nil)
	rec := httptest.NewRecorder()
	server.New(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/dev/stream-test without devroutes: got %d, want 404", rec.Code)
	}
}

// Without devroutes the server's clock cannot be moved (B-4).
func TestM2_DevClockIsAbsentWithoutTheTag(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/dev/clock", nil)
	rec := httptest.NewRecorder()
	server.New(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/dev/clock without devroutes: got %d, want 404", rec.Code)
	}
}

// B-4, E.4: on the server the public image runs, with its owner middleware
// and route table, nothing under /api/dev/ exists, for any visitor. Only the
// web page's GET / answers other methods, with 405, and no route exists for
// any of these paths.
func TestB4_NoDevRouteExistsWithoutTheTag(t *testing.T) {
	h, db, _ := newAPI(t)
	bare := server.NewWithAPI(nil, server.NewHub(), db, nil)
	paths := []string{"/api/dev/stream-test", "/api/dev/clock", "/api/dev/", "/api/dev/anything"}
	for _, handler := range []http.Handler{h, bare} {
		for _, path := range paths {
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				req := httptest.NewRequest(method, path, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				absent := rec.Code == http.StatusNotFound ||
					(method != http.MethodGet && rec.Code == http.StatusMethodNotAllowed)
				if !absent {
					t.Fatalf("%s %s without devroutes: got %d, want it absent", method, path, rec.Code)
				}
			}
		}
	}
}
