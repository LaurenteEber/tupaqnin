package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedCriticalDecision adds the agent's dec-1 used in sec-1 (a
// conclusion), inside a transaction that reconciles criticality, so
// card-1 opens (T-5).
func seedCriticalDecision(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		return cycle.TrackCriticality(ctx, tx, "", func() error {
			sec, err := tx.GetObject(ctx, projectID, "sec-1")
			if err != nil {
				return err
			}
			dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
				"statement": "Comparar por veces.", "instead_of": "Comparar conteos crudos.",
				"would_choose_if": "Todos usaran el mismo filtro.", "origin": "agent"},
				store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
			if err != nil {
				return err
			}
			_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", store.EventInput{Type: store.EventEdge, Actor: "agent"})
			return err
		})
	})
	if err != nil {
		t.Fatalf("seed decision: %v", err)
	}
}

func TestP4_TodoListsTheOpenCardAndAnAssentLeavesItThere(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	base := "/api/projects/" + projectID

	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.Cards) != 1 || todo.Cards[0].Label != "card-1" || todo.Cards[0].Decision != "dec-1" ||
		len(todo.Cards[0].Reasons) == 0 || todo.Cards[0].Reasons[0] != "conclusion_or_recommendation_section" ||
		len(todo.Cards[0].Sections) != 1 || todo.Cards[0].Sections[0].Title != "Síntesis" {
		t.Fatalf("todo: %+v", todo)
	}
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "ok"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("answer with ok: %d (%s), want 422", rec.Code, rec.Body.String())
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 1 {
		t.Fatalf("todo after ok: %+v, want the card still open", todo)
	}
	rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "other", "reason": "Porque el comité pidió conteos crudos."})
	if got := decode[server.AnswerCardResponse](t, rec); rec.Code != http.StatusOK || got.NewDecision != "dec-2" {
		t.Fatalf("answer other: %d %+v", rec.Code, got)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 0 {
		t.Fatalf("todo after answer: %+v, want empty", todo)
	}
	if rec := do(t, h, http.MethodGet, base+"/cards/card-1", nil); decode[server.CardView](t, rec).State != "answered" {
		t.Fatalf("card-1: %s", rec.Body.String())
	}
}

// K-13: changing a section's role to conclusion through the route makes
// its agent decision critical and opens the card.
func TestK13_RoleChangeThroughTheRouteOpensTheCard(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "other"}); rec.Code != http.StatusOK {
		t.Fatalf("role other: %d", rec.Code)
	}
	seedCriticalDecision(t, db, projectID) // used only in sec-1, now role other: not critical
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 0 {
		t.Fatalf("todo before: %+v", todo)
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "recommendation"}); rec.Code != http.StatusOK {
		t.Fatalf("role recommendation: %d", rec.Code)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 1 {
		t.Fatalf("todo after role change: %+v, want one open card", todo)
	}
}

// P-36: the card names its sections by title, never by label.
func TestP36_CardNamesItsSectionsByTitle(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	card := decode[server.CardView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/cards/card-1", nil))
	if len(card.Sections) != 1 || card.Sections[0].Title != "Síntesis" {
		t.Fatalf("card sections: %+v, want the title Síntesis", card.Sections)
	}
}
