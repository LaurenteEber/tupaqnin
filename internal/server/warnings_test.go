package server_test

import (
	"context"
	"net/http"
	"testing"
	"unicode/utf16"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.9 (backlog-r2) «Avisos en el texto» through GET
// /sections/{label}: each warning with its kind, the literal passage
// where it goes (if any) and the label it leads to. Go computes where
// it sits (block and offsets); the page only underlines it.

func warningsOf(t *testing.T, h http.Handler, base, label string) []graph.Warning {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/sections/"+label, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", label, rec.Code, rec.Body.String())
	}
	return decode[server.SectionDetail](t, rec).Warnings
}

// utf16Slice cuts a block by the UTF-16 offsets the page uses.
func utf16Slice(s string, start, end int) string {
	u := utf16.Encode([]rune(s))
	if start < 0 || end > len(u) || start > end {
		return "<out of range>"
	}
	return string(utf16.Decode(u[start:end]))
}

// A-1, E-4: a stale section brings its open change, with its label, as a
// warning of kind stale; what changed is named by its words (the source's
// file name), never by its label. A section that does not use the source
// has none; closing the change from the band's route clears it.
func TestA1_StaleSectionDetailBringsItsChangeWithItsLabel(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	upload(t, h, base, "calculation", "calculo.md", []byte("Promedio: 60."))
	seedSectionsUsing(t, db, projectID)
	uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 61."))

	changes := decode[server.ChangesView](t, do(t, h, http.MethodGet, base+"/sections/sec-1/changes", nil))
	if changes.State != "stale" || len(changes.Changes) != 1 {
		t.Fatalf("sec-1 changes: %+v, want stale with one", changes)
	}
	got := warningsOf(t, h, base, "sec-1")
	if len(got) != 1 {
		t.Fatalf("sec-1 warnings: %+v, want one", got)
	}
	w := got[0]
	if w.Kind != graph.WarningStale || w.Target != changes.Changes[0].Label || w.ChangeKind != "source" ||
		w.What != "calculo.md" || w.Passage != "" || w.Block != 0 {
		t.Fatalf("sec-1 warning: %+v, want stale → %s about calculo.md, no passage", w, changes.Changes[0].Label)
	}
	if got := warningsOf(t, h, base, "sec-3"); len(got) != 0 {
		t.Fatalf("sec-3 warnings: %+v, want none", got)
	}

	if rec := do(t, h, http.MethodPost, base+"/changes/"+w.Target+"/close",
		map[string]any{"result": "reviewed_unchanged", "reason": "Porque la síntesis no usa el promedio."}); rec.Code != http.StatusNoContent {
		t.Fatalf("close: %d %s", rec.Code, rec.Body.String())
	}
	if got := warningsOf(t, h, base, "sec-1"); len(got) != 0 {
		t.Fatalf("after closing: %+v, want none", got)
	}
	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if detail.Status != "ready" {
		t.Fatalf("after closing: %s, want ready", detail.Status)
	}
}

// Estados §1: in a signed section the same change moves it to «cambió
// desde que la firmaste» and enters its list: the warning is of kind
// changed.
func TestE1_ChangeOnASignedSectionIsAChangedWarning(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	upload(t, h, base, "calculation", "calculo.md", []byte("Promedio: 60."))
	seedSectionsUsing(t, db, projectID)
	ctx := context.Background()
	if err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-2")
		if err != nil {
			return err
		}
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", sec.Body,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("sign sec-2: %v", err)
	}
	uploadReplacing(t, h, base, "src-1", "calculo.md", []byte("Promedio: 61."))

	changes := decode[server.ChangesView](t, do(t, h, http.MethodGet, base+"/sections/sec-2/changes", nil))
	got := warningsOf(t, h, base, "sec-2")
	if changes.State != "changed_since_signoff" || len(changes.Changes) != 1 || len(got) != 1 ||
		got[0].Kind != graph.WarningChanged || got[0].Target != changes.Changes[0].Label {
		t.Fatalf("sec-2: %+v, warnings %+v; want changed → its change", changes, got)
	}
}

// seedSectionWithCode creates dec-1 and sec-1, whose second paragraph
// names dec-1 with an upper case and a non-ASCII dash, as a model could.
func seedSectionWithCode(t *testing.T, db *store.DB, projectID, text string) {
	t.Helper()
	ctx := context.Background()
	if err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Comparar por promedio."},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Síntesis", "text": text,
			"role": map[string]any{"value": "other", "state": "confirmed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// T-2, B-15: a pending «códigos internos» brings its passage, literal as
// written, with its block and its offsets in the paragraph the editor
// shows for that block (a heading keeps its «## » there); it leads to the
// section (its history). When the code leaves the text, the warning goes.
func TestT2_InternalCodesPendingBringsItsPassage(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	block2 := "## Según la DEC‑1, el promedio de «ñandú» sube"
	seedSectionWithCode(t, db, projectID, "Primer párrafo.\n\n"+block2)

	var w graph.Warning
	for _, got := range warningsOf(t, h, base, "sec-1") {
		if got.Kind == graph.WarningAgentPending && got.Pending == graph.PendingInternalCodes {
			w = got
		}
	}
	if w.Passage != "DEC‑1" || w.Block != 2 || w.Target != "sec-1" {
		t.Fatalf("internal codes warning: %+v, want passage «DEC‑1» in block 2 → sec-1", w)
	}
	if cut := utf16Slice(block2, w.Start, w.End); cut != "DEC‑1" {
		t.Fatalf("offsets %d..%d cut %q, want «DEC‑1»", w.Start, w.End, cut)
	}

	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	claimEditorTab(t, h, base)
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{
		"tab_id": "tab-1", "base_version": detail.Version, "text": "Primer párrafo.\n\nEl promedio sube.",
	}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	for _, got := range warningsOf(t, h, base, "sec-1") {
		if got.Pending == graph.PendingInternalCodes {
			t.Fatalf("after removing the code: %+v, want no internal codes warning", got)
		}
	}
}

// P4 (pantallas, 2.4 · 2.9): an open «por aclarar» whose passage is in
// the text brings it underlined (block and offsets) and leads to its
// label. If the passage leaves the text, the question is still open and
// still warns, without a place.
func TestP4_OpenQuestionPassageIsPlacedAndLeadsToItsLabel(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	block1 := "El piloto D1 fue un estudio de campo."
	seedSectionWithCode(t, db, projectID, block1+"\n\nOtro párrafo.")
	ctx := context.Background()
	if err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		_, err = checks.Open(ctx, tx, checks.OpenQuestion{
			Question:  "¿El piloto D1 cuenta como estudio de campo?",
			Options:   []map[string]any{{"option": "Sí"}, {"option": "No"}},
			DependsOn: []string{sec.ID},
			Extra:     map[string]any{"origin": "checker", "new_passage": "un estudio de campo"},
			Event:     store.EventInput{Type: store.EventStateChanged, Actor: "app"},
		})
		return err
	}); err != nil {
		t.Fatalf("open question: %v", err)
	}

	var w graph.Warning
	for _, got := range warningsOf(t, h, base, "sec-1") {
		if got.Kind == graph.WarningOpenQuestion {
			w = got
		}
	}
	if w.Target != "oq-1" || w.Passage != "un estudio de campo" || w.Block != 1 ||
		utf16Slice(block1, w.Start, w.End) != "un estudio de campo" ||
		w.What != "¿El piloto D1 cuenta como estudio de campo?" {
		t.Fatalf("open question warning: %+v, want oq-1 at «un estudio de campo» in block 1", w)
	}

	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	claimEditorTab(t, h, base)
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{
		"tab_id": "tab-1", "base_version": detail.Version, "text": "El piloto D1 fue una prueba.\n\nOtro párrafo.",
	}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	w = graph.Warning{}
	for _, got := range warningsOf(t, h, base, "sec-1") {
		if got.Kind == graph.WarningOpenQuestion {
			w = got
		}
	}
	if w.Target != "oq-1" || w.Passage != "" || w.Block != 0 {
		t.Fatalf("after the passage left: %+v, want oq-1 without a place", w)
	}
}

// Kept behaviour: the section detail still carries its state, pendings
// and provenance beside the warnings (2.1, 2.11).
func TestB15_SectionDetailKeepsItsPendingsBesideTheWarnings(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSectionWithCode(t, db, projectID, "Primer párrafo.\n\nSegún la DEC‑1, sube.")
	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	found := false
	for _, p := range detail.Pendings {
		if p.Kind == graph.PendingInternalCodes && p.About == "dec-1" {
			found = true
		}
	}
	if !found || detail.Status != "draft" || len(detail.Provenance) != 2 {
		t.Fatalf("detail: %+v, want draft with the internal codes pending and two paragraphs", detail)
	}
}

func claimEditorTab(t *testing.T, h http.Handler, base string) {
	t.Helper()
	if rec := do(t, h, http.MethodPost, base+"/editor", map[string]any{"tab_id": "tab-1"}); rec.Code >= 300 {
		t.Fatalf("claim editor: %d %s", rec.Code, rec.Body.String())
	}
}
