package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.New().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET /healthz: got Content-Type %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("GET /healthz: decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("GET /healthz: got status field %q, want %q", body["status"], "ok")
	}
}

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "health endpoint", path: "/healthz"},
		{name: "index page", path: "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			server.New().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: got status %d, want %d", tt.path, rec.Code, http.StatusOK)
			}
			wantHeaders := map[string]string{
				"Content-Security-Policy": "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'",
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
			}
			for header, want := range wantHeaders {
				if got := rec.Header().Get(header); got != want {
					t.Errorf("GET %s: header %s = %q, want %q", tt.path, header, got, want)
				}
			}
		})
	}
}

func TestIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.New().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "tupaqnin") {
		t.Fatalf("GET /: body does not mention tupaqnin: %q", rec.Body.String())
	}
}
