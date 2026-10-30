package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.1 (backlog-r2) through the routes.

// seedCoveredFigure gives sec-1 a text with «60 veces», a figure on it
// and covers to dec-1, the decision seedCriticalDecision left critical.
func seedCoveredFigure(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		dec, err := tx.GetObject(ctx, projectID, "dec-1")
		if err != nil {
			return err
		}
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, map[string]any{
			"title": "Síntesis", "text": "La embotellada trae 60 veces más.",
			"role": map[string]any{"value": "conclusion", "state": "proposed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventEdit, Actor: "agent"}); err != nil {
			return err
		}
		fig, err := tx.CreateObject(store.ObjectFigure, "",
			map[string]any{"section_id": sec.ID, "anchor": "60 veces", "value": "60"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeCovers, fig.ID, dec.ID, "60 veces", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed figure: %v", err)
	}
}

// B-49 and B-36: your save that changes a critical figure opens the R4
// card through PUT /sections/{label}; it is answered with option null
// and refuses an option.
func TestB49_YourSaveOnACriticalFigureOpensTheR4CardAnsweredWithNullOption(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "Porque compara igual."}); rec.Code != http.StatusOK {
		t.Fatalf("answer card-1: %d %s", rec.Code, rec.Body.String())
	}
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": sec.CurrentVersion, "text": "La embotellada trae 61 veces más."}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.Cards) != 1 || todo.Cards[0].Label != "card-2" || !todo.Cards[0].OwnEdit || todo.Cards[0].Decision != "dec-1" {
		t.Fatalf("todo: %+v", todo)
	}
	if rec := do(t, h, http.MethodPost, base+"/cards/card-2/answer", map[string]any{"option": "keep", "reason": "Porque cambió la cifra."}); rec.Code != http.StatusBadRequest {
		t.Fatalf("answer with an option: %d, want 400", rec.Code)
	}
	rec := do(t, h, http.MethodPost, base+"/cards/card-2/answer", map[string]any{"option": nil, "reason": "Porque el estudio corrigió la cifra."})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer with null option: %d %s", rec.Code, rec.Body.String())
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 0 {
		t.Fatalf("todo after answer: %+v", todo)
	}
}

// B-36: a save that keeps the fingerprint runs no rule (and writes
// nothing).
func TestB36_SaveWithTheSameFingerprintOpensNothing(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": sec.CurrentVersion, "text": "La embotellada trae 60 veces más."}); rec.Code != http.StatusNoContent {
		t.Fatalf("save: %d, want 204", rec.Code)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 1 || todo.Cards[0].OwnEdit {
		t.Fatalf("todo: %+v, want only card-1", todo)
	}
}

// T-5 and specification §1.5: PUT /decisions/{label}/critical marks a
// decision (its card opens when it is the agent's) and unmarks it, each
// with marked_critical.
func TestT5_MarkCriticalRouteOpensTheCardAndUnmarkingIsRecorded(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Comparar por veces.", "instead_of": "Comparar conteos.", "would_choose_if": "Mismo filtro.",
			"origin": "agent", "human_marked_critical": false}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if rec := do(t, h, http.MethodPut, base+"/decisions/dec-1/critical", map[string]any{"marked": true}); rec.Code != http.StatusNoContent {
		t.Fatalf("mark: %d %s", rec.Code, rec.Body.String())
	}
	if critical, reasons, _ := graph.Critical(context.Background(), db, projectID, "dec-1"); !critical || reasons[0] != graph.ReasonHumanMarked {
		t.Fatalf("dec-1: %v %v", critical, reasons)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 1 || todo.Cards[0].Decision != "dec-1" {
		t.Fatalf("todo: %+v", todo)
	}
	if rec := do(t, h, http.MethodPut, base+"/decisions/dec-1/critical", map[string]any{"marked": false}); rec.Code != http.StatusNoContent {
		t.Fatalf("unmark: %d", rec.Code)
	}
	events, _ := db.EventsSince(context.Background(), projectID, 0)
	var marks int
	for _, ev := range events {
		if ev.Type == store.EventMarkedCritical {
			marks++
		}
	}
	if marks != 2 {
		t.Fatalf("marked_critical: %d, want 2", marks)
	}
	if rec := do(t, h, http.MethodPut, base+"/decisions/dec-9/critical", map[string]any{"marked": true}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown decision: %d, want 404", rec.Code)
	}
}

// B-43: the conversation shows the reclassification with the triage line.
func TestB43_ConversationCarriesTheReclassification(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal",
			Data: map[string]any{"turn_kind": "step", "triage_line": "Vía directa.", "stretch": "build",
				"proposed_stretch": "direct", "reclassified": []any{"figure"}}})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	entries := decode[[]server.ConversationEntry](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/conversation", nil))
	if len(entries) != 1 || entries[0].Stretch != "build" || len(entries[0].Reclassified) != 1 || entries[0].Reclassified[0] != "figure" {
		t.Fatalf("conversation: %+v", entries)
	}
}
