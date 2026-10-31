package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// heldReaders holds every reader call until release is closed.
type heldReaders struct {
	inner   provider.Provider
	release chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (h *heldReaders) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnReader {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
	return h.inner.Respond(ctx, req, onDelta)
}

// B-72, B-64, P-20: POST /rounds creates the round from your act (the app
// sets the reason; parts openalex and, with material, project; never
// arxiv), takes the turn slot (a message gets 409 until it ends), and
// GET /rounds/{label} shows its card; the conversation lists it.
func TestB72_PostRoundsCreatesTheRoundAndTakesTheTurnSlot(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant", "content": []map[string]string{{
		"type": "output_text", "text": `{"findings":[],"covered":[],"sources_read":[]}`}}})
	fake := provider.NewFake(provider.Fixture{TurnKind: provider.TurnReader, Items: []json.RawMessage{item}})
	fake.Recorder = db
	held := &heldReaders{inner: fake, release: make(chan struct{}), entered: make(chan struct{})}
	hub := server.NewHub()
	runner := cycle.NewRunner(db, held, nil, hub)
	runner.Evidence = evidence.NewReplay()
	h := server.NewWithAPI(nil, hub, db, runner)
	projectID := newProject(t, h, nil)
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": "Busca evidencia."}, human); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "a.md", "text": "Texto."}, human)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	base := "/api/projects/" + projectID

	rec := do(t, h, http.MethodPost, base+"/rounds", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /rounds: %d %s", rec.Code, rec.Body.String())
	}
	started := decode[server.StartRoundResponse](t, rec)
	if started.Label != "rnd-1" || started.TurnID == "" {
		t.Fatalf("started: %+v", started)
	}
	<-held.entered
	if rec := do(t, h, http.MethodPost, base+"/messages", map[string]string{"text": "hola"}); rec.Code != http.StatusConflict {
		t.Fatalf("message during the round: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/rounds", nil); rec.Code != http.StatusConflict {
		t.Fatalf("second round: %d", rec.Code)
	}
	view := decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/rnd-1", nil))
	if !view.Running || view.DelegationReason != "independent_parts" || len(view.Parts) != 2 ||
		view.Parts[0].Where[0] != "openalex" || view.Parts[1].Where[0] != "project" {
		t.Fatalf("card: %+v", view)
	}
	for _, p := range view.Parts {
		for _, w := range p.Where {
			if w == "arxiv" {
				t.Fatal("a round you start never searches arxiv")
			}
		}
	}
	close(held.release)
	deadline := time.Now().Add(10 * time.Second)
	for view.Running || view.Parts[0].Status != cycle.ReaderReturned {
		if time.Now().After(deadline) {
			t.Fatalf("round did not end: %+v", view)
		}
		time.Sleep(10 * time.Millisecond)
		view = decode[server.RoundCard](t, do(t, h, http.MethodGet, base+"/rounds/rnd-1", nil))
	}
	runner.Close()
	entries := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, base+"/conversation", nil))
	found := false
	for _, e := range entries {
		found = found || (e.Kind == "round" && e.Label == "rnd-1")
	}
	if !found {
		t.Fatalf("conversation: %+v", entries)
	}
	if rec := do(t, h, http.MethodGet, base+"/rounds/req-1", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("a label that is not a round: %d", rec.Code)
	}
}
