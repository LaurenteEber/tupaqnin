//go:build devroutes

package server_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/server"
)

// With -tags=devroutes the test route exists (B-4). The full 90 s
// stream takes too long for this check; it only proves the route
// answers as an event stream and starts emitting deltas.
func TestB4_DevStreamRouteExistsWithTheTag(t *testing.T) {
	handler := server.New(nil)
	w := newSyncWriter()
	done := serveAsync(t, handler, "/api/dev/stream-test", w)

	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(w.String(), "event: delta") {
		if time.Now().After(deadline) {
			t.Fatalf("dev stream emitted no delta; body:\n%s", w.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type: got %q, want text/event-stream", ct)
	}
	_ = done // the request context ends with the test, which stops the fake
}
