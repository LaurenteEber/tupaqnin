package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.3: GET /sources/{label} is a new route. An external
// (web) source carries url, doi, title, authors, year and access
// (B-61); your material keeps its view, its download still works, and
// the request lists only your material.
func TestB61_GetSourceShowsAWebSourceWithItsRecord(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	hub := server.NewHub()
	runner := cycle.NewRunner(db, provider.NewFake(), nil, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	projectID := newProject(t, h, nil)
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "a.md",
			"media_type": "text/markdown", "text": "Texto.", "fingerprint": "fa"},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "web", "url": "https://openalex.example/W1",
			"media_type": "text/plain", "text": "Micro-FTIR detects 20 µm.", "fingerprint": "fw", "doi": "10.1/limits",
			"title": "Detection limits", "authors": []string{"Ana Ruiz", "Luis Paz"}, "year": 2021, "access": "abstract"},
			store.EventInput{Type: store.EventStateChanged, Actor: "app"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	base := "/api/projects/" + projectID

	rec := do(t, h, http.MethodGet, base+"/sources/src-2", nil)
	web := decode[server.SourceView](t, rec)
	if rec.Code != http.StatusOK || web.Kind != "web" || web.URL != "https://openalex.example/W1" || web.DOI != "10.1/limits" ||
		web.Title != "Detection limits" || len(web.Authors) != 2 || web.Year != 2021 || web.Access != "abstract" {
		t.Fatalf("web source: %d %+v", rec.Code, web)
	}
	rec = do(t, h, http.MethodGet, base+"/sources/src-1", nil)
	body := rec.Body.String()
	want := `{"label":"src-1","version":1,"kind":"file","filename":"a.md","media_type":"text/markdown","fingerprint":"fa"}` + "\n"
	if rec.Code != http.StatusOK || body != want {
		t.Fatalf("file source: %d %s", rec.Code, body)
	}
	if rec := do(t, h, http.MethodGet, base+"/sources/src-9", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing source: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, base+"/sources/src-1/download", nil); rec.Code != http.StatusOK || rec.Body.String() != "Texto." {
		t.Fatalf("download: %d %q", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, base+"/request", nil)
	view := decode[server.RequestView](t, rec)
	if len(view.Sources) != 1 || view.Sources[0].Label != "src-1" {
		t.Fatalf("request sources: %+v", view.Sources)
	}
}
