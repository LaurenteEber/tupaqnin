package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-100: POST /rounds on a project with no request says so: 422 with its
// code, not 500, and again on a second try (the slot is free).
func TestP100_PostRoundsWithoutARequestIs422(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := provider.NewFake()
	fake.Recorder = db
	hub := server.NewHub()
	runner := cycle.NewRunner(db, fake, nil, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	base := "/api/projects/" + newProject(t, h, nil)
	for range 2 {
		rec := do(t, h, http.MethodPost, base+"/rounds", nil)
		var body struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != http.StatusUnprocessableEntity || body.Error != "request_required" {
			t.Fatalf("POST /rounds: %d %s", rec.Code, rec.Body.String())
		}
	}
}

// conflictRunner is a runner whose «dejarlo por aclarar» loses the race.
type conflictRunner struct{ *cycle.Runner }

func (conflictRunner) LeaveRoundOpen(context.Context, string, string) (string, error) {
	return "", store.ErrVersionConflict
}

// P-100: a version conflict in «dejarlo por aclarar» is 409 with its code.
func TestP100_LeaveOpenVersionConflictIs409(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := provider.NewFake()
	fake.Recorder = db
	hub := server.NewHub()
	runner := cycle.NewRunner(db, fake, nil, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, conflictRunner{runner}))
	base := "/api/projects/" + newProject(t, h, nil)
	rec := do(t, h, http.MethodPost, base+"/rounds/rnd-1/leave-open", nil)
	var body struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusConflict || body.Error != "version_conflict" {
		t.Fatalf("leave-open: %d %s", rec.Code, rec.Body.String())
	}
}
