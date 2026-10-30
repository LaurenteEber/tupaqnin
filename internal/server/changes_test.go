package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.2 (backlog-r2) through the routes.

func uploadReplacing(t *testing.T, h http.Handler, base, replaces, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kind", "calculation")
	_ = mw.WriteField("replaces", replaces)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, base+"/sources", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// seedSectionsUsing creates sec-1 and sec-2, both using src-1 with what
// they used (B-45), and sec-3, which does not use it.
func seedSectionsUsing(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		src, err := tx.GetObject(ctx, projectID, "src-1")
		if err != nil {
			return err
		}
		for i, title := range []string{"Síntesis", "Conclusiones", "Alcance"} {
			sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
				"title": title, "text": "Texto de " + title + ".",
				"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
			}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
			if err != nil {
				return err
			}
			if i < 2 {
				if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", graph.UsageOf(src),
					store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed sections: %v", err)
	}
}

func countEvents(t *testing.T, db *store.DB, projectID string, typ store.EventType) int {
	t.Helper()
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	n := 0
	for _, ev := range events {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

// A-1, B-9: uploading the calculation with replaces is the new version
// of the same source and warns each section that uses it; uploading it
// identical does not.
func TestA1_ReplacedSourceWarnsEachSectionThatUsesItAndAnIdenticalOneDoesNot(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := upload(t, h, base, "calculation", "calculo.md", []byte("Promedio: 60.")); rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	seedSectionsUsing(t, db, projectID)

	rec := uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 60."))
	if rec.Code != http.StatusCreated || decode[server.SourceView](t, rec).Version != 1 {
		t.Fatalf("identical upload: %d %s, want version 1 unchanged", rec.Code, rec.Body.String())
	}
	if n := countEvents(t, db, projectID, store.EventSourceNewVersion); n != 0 {
		t.Fatalf("source_new_version after an identical upload: %d", n)
	}

	rec = uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 61."))
	view := decode[server.SourceView](t, rec)
	if rec.Code != http.StatusCreated || view.Label != "src-1" || view.Version != 2 {
		t.Fatalf("replace: %d %+v, want src-1 version 2", rec.Code, view)
	}
	if n := countEvents(t, db, projectID, store.EventSourceNewVersion); n != 1 {
		t.Fatalf("source_new_version: %d, want 1", n)
	}
	for label, want := range map[string]int{"sec-1": 1, "sec-2": 1, "sec-3": 0} {
		got := decode[server.ChangesView](t, do(t, h, http.MethodGet, base+"/sections/"+label+"/changes", nil))
		if len(got.Changes) != want {
			t.Fatalf("%s changes: %+v, want %d", label, got, want)
		}
		if want == 1 && (got.State != "stale" || got.Changes[0].About != "src-1" || got.Changes[0].Kind != "source") {
			t.Fatalf("%s: %+v, want stale with the change of src-1", label, got)
		}
	}
	if rec := uploadReplacing(t, h, base, "src-9", "calculo.md", []byte("x")); rec.Code != http.StatusNotFound {
		t.Fatalf("replaces an unknown source: %d, want 404", rec.Code)
	}
}

// T-1 and A-1 through POST /changes/{label}/close: «ok» does not
// close (422); a valid «porque» does, and the empty list returns the
// section to draft.
func TestT1_CloseRouteRefusesOkAndClosesWithAReason(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	upload(t, h, base, "calculation", "calculo.md", []byte("Promedio: 60."))
	seedSectionsUsing(t, db, projectID)
	uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 61."))

	changes := decode[server.ChangesView](t, do(t, h, http.MethodGet, base+"/sections/sec-1/changes", nil))
	label := changes.Changes[0].Label
	if rec := do(t, h, http.MethodPost, base+"/changes/"+label+"/close",
		map[string]any{"result": "reviewed_unchanged", "reason": "ok"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("close with ok: %d, want 422", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/changes/"+label+"/close",
		map[string]any{"result": "updated"}); rec.Code != http.StatusConflict {
		t.Fatalf("updated without an edit: %d, want 409", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/changes/chg-99/close",
		map[string]any{"result": "reviewed_unchanged", "reason": "Porque no usa el promedio."}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown change: %d, want 404", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/changes/"+label+"/close",
		map[string]any{"result": "reviewed_unchanged", "reason": "Porque la síntesis no usa el promedio."}); rec.Code != http.StatusNoContent {
		t.Fatalf("close with a reason: %d %s", rec.Code, rec.Body.String())
	}
	got := decode[server.ChangesView](t, do(t, h, http.MethodGet, base+"/sections/sec-1/changes", nil))
	if len(got.Changes) != 0 || got.State != "ready" {
		t.Fatalf("after closing: %+v, want ready with no changes", got)
	}
	var data map[string]any
	events, _ := db.EventsSince(context.Background(), projectID, 0)
	for _, ev := range events {
		if ev.Type == store.EventUsageChange {
			_ = json.Unmarshal(ev.Data, &data)
		}
	}
	if data["kind"] != "reviewed_unchanged" || data["reason"] != "Porque la síntesis no usa el promedio." {
		t.Fatalf("last usage_change: %v", data)
	}
}

// E-4 through the route: with a change open, asking to sign lists it
// and opens no signoff.
func TestE4_RequestSignoffWithAnOpenChangeListsIt(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	upload(t, h, base, "calculation", "calculo.md", []byte("Promedio: 60."))
	seedSectionsUsing(t, db, projectID)
	uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 61."))
	rec := do(t, h, http.MethodPost, base+"/sections/sec-2/signoff", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("signoff: %d %s, want 409", rec.Code, rec.Body.String())
	}
	if got := decode[server.SignoffBlocked](t, rec); len(got.OpenChanges) != 1 {
		t.Fatalf("signoff blocked: %+v, want one open change", got)
	}
}
