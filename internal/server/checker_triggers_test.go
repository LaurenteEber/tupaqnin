package server_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// countChecker counts the checker's calls.
type countChecker struct {
	inner provider.Provider
	mu    sync.Mutex
	calls int
}

func (c *countChecker) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	if req.TurnKind == provider.TurnChecker {
		c.mu.Lock()
		c.calls++
		c.mu.Unlock()
	}
	return c.inner.Respond(ctx, req, onDelta)
}

// B-36 through the routes: a save of your edit that changes the section
// starts the 60 s wait; a save without change does not; asking to sign
// sends the pending batch before the signoff opens.
func TestB36_SaveStartsTheWaitAndAskingToSignRunsTheBatch(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	prov := &countChecker{inner: provider.NewFake()}
	hub := server.NewHub()
	runner := cycle.NewRunner(db, prov, nil, hub)
	var mu sync.Mutex
	var waits []time.Duration
	runner.AfterFunc = func(d time.Duration, f func()) func() bool {
		mu.Lock()
		defer mu.Unlock()
		waits = append(waits, d)
		return func() bool { return true }
	}
	t.Cleanup(runner.Close)
	h := server.NewWithAPI(nil, hub, db, runner)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, plainText, false)

	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": plainText}); rec.Code != http.StatusNoContent {
		t.Fatalf("save without change: %d", rec.Code)
	}
	if len(waits) != 0 {
		t.Fatalf("waits: %v, want none for a save without change", waits)
	}
	edited := plainText + " Y nada más."
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": edited}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if len(waits) != 1 || waits[0] != cycle.EditBatchDelay {
		t.Fatalf("waits: %v, want one of 60 s", waits)
	}

	openSignoffRoute(t, h, base)
	if prov.calls != 1 {
		t.Fatalf("checker calls: %d, want the batch of your edit before the signoff", prov.calls)
	}
}
