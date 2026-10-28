package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
)

// syncWriter is a ResponseWriter safe to read while the handler runs.
type syncWriter struct {
	mu     sync.Mutex
	header http.Header
	body   strings.Builder
	status int
}

func newSyncWriter() *syncWriter { return &syncWriter{header: http.Header{}} }
func (w *syncWriter) Header() http.Header {
	return w.header
}
func (w *syncWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}
func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(p)
}
func (w *syncWriter) Flush() {}
func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

// serveAsync starts the stream and returns when the handler finishes.
func serveAsync(t *testing.T, handler http.Handler, path string, w *syncWriter) chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(w, req)
	}()
	return done
}

func waitFor(t *testing.T, done chan struct{}, timeout time.Duration, what string, w *syncWriter) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s; body so far:\n%s", what, w.String())
	}
}

func deltaTexts(t *testing.T, body string) []string {
	t.Helper()
	var texts []string
	for _, block := range strings.Split(body, "\n\n") {
		if !strings.HasPrefix(block, "event: delta\n") {
			continue
		}
		data := strings.TrimPrefix(block, "event: delta\ndata: ")
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			t.Fatalf("decode delta %q: %v", data, err)
		}
		texts = append(texts, payload.Text)
	}
	return texts
}

func TestB1_LateSubscriberReceivesEverythingFromTheStart(t *testing.T) {
	hub := server.NewHub()
	hub.HeartbeatInterval = 10 * time.Millisecond
	handler := server.NewWithHub(nil, hub)

	// The turn emits before anyone connects.
	hub.PublishDelta("p1", "t1", "one ")
	hub.PublishDelta("p1", "t1", "two ")

	w := newSyncWriter()
	done := serveAsync(t, handler, "/api/projects/p1/turns/t1/stream", w)

	hub.PublishDelta("p1", "t1", "three")
	hub.PublishStep("p1", "t1", json.RawMessage(`{"status":"accepted"}`))
	waitFor(t, done, 5*time.Second, "stream to finish", w)

	texts := deltaTexts(t, w.String())
	if strings.Join(texts, "") != "one two three" {
		t.Fatalf("late subscriber deltas: got %v, want [one two three] in order", texts)
	}
	if !strings.Contains(w.String(), "event: step\ndata: {\"status\":\"accepted\"}") {
		t.Fatalf("stream does not end with the step result:\n%s", w.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type: got %q, want text/event-stream", ct)
	}
}

func TestB1_Stream90ArrivesCompleteWithHeartbeats(t *testing.T) {
	fixture, err := provider.Stream90Fixture()
	if err != nil {
		t.Fatalf("Stream90Fixture: %v", err)
	}
	// Same 90 deltas; the rhythm is compressed so the test does not
	// take 90 s. The fixture itself stays at 1 s per delta (0.5).
	fixture.DeltaDelay = 5 * time.Millisecond

	hub := server.NewHub()
	hub.HeartbeatInterval = 15 * time.Millisecond
	handler := server.NewWithHub(nil, hub)

	fake := provider.NewFake(fixture)
	go func() {
		resp, err := fake.Respond(context.Background(), provider.Request{
			ProjectID: "p1", TurnID: "t90", Model: provider.PrincipalModel,
		}, func(d provider.Delta) { hub.PublishDelta("p1", "t90", d.Text) })
		if err != nil {
			hub.PublishError("p1", "t90", "provider_error")
			return
		}
		step, _ := json.Marshal(map[string]string{"status": "completed", "text": resp.Text})
		hub.PublishStep("p1", "t90", step)
	}()

	w := newSyncWriter()
	done := serveAsync(t, handler, "/api/projects/p1/turns/t90/stream", w)
	waitFor(t, done, 10*time.Second, "90-delta stream", w)

	body := w.String()
	texts := deltaTexts(t, body)
	if len(texts) != 90 {
		t.Fatalf("deltas: got %d, want 90", len(texts))
	}
	for i, text := range texts {
		if want := fmt.Sprintf("word-%d ", i+1); text != want {
			t.Fatalf("delta %d: got %q, want %q", i, text, want)
		}
	}
	if !strings.Contains(body, ": heartbeat") {
		t.Fatalf("stream has no heartbeat:\n%s", body)
	}
	if !strings.Contains(body, "event: step") {
		t.Fatalf("stream does not close with a step event:\n%s", body)
	}
}

func TestB1_ErrorEventCarriesOnlyCodeAndCatalogMessage(t *testing.T) {
	hub := server.NewHub()
	handler := server.NewWithHub(nil, hub)
	hub.PublishError("p1", "t1", "provider_error")

	w := newSyncWriter()
	done := serveAsync(t, handler, "/api/projects/p1/turns/t1/stream", w)
	waitFor(t, done, 5*time.Second, "error stream", w)

	body := w.String()
	if !strings.Contains(body, "event: error") || !strings.Contains(body, `"code":"provider_error"`) {
		t.Fatalf("error event missing code:\n%s", body)
	}
	if strings.Contains(body, "raw") && strings.Contains(body, "stack") {
		t.Fatalf("error event leaks raw detail:\n%s", body)
	}
}
