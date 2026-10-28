package server_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

func TestT10_RequestLogDoesNotContainBody(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	const secret = "document-text-that-must-not-be-logged"
	req := httptest.NewRequest(http.MethodPost, "/healthz", strings.NewReader(secret))
	rec := httptest.NewRecorder()

	server.New(logger).ServeHTTP(rec, req)

	log := buf.String()
	if log == "" {
		t.Fatal("request log: got no output, want a JSON request line")
	}
	if strings.Contains(log, secret) {
		t.Fatalf("request log contains the request body: %s", log)
	}
	if !strings.Contains(log, `"method":"POST"`) || !strings.Contains(log, `"path":"/healthz"`) {
		t.Fatalf("request log missing method/path: %s", log)
	}
}
