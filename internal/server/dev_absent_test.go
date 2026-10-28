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
