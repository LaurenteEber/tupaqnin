package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// seedUnderstanding inserts one case and one section with a proposed
// role, as an accepted understanding turn would have created them.
func seedUnderstanding(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectUnderstandingCase, "unconfirmed",
			map[string]any{"text": "Un muestreo de grifos reales.", "counts": true, "criterion_id": ""},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"}); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSection, "draft",
			map[string]any{"title": "Síntesis", "text": "",
				"role": map[string]any{"value": "conclusion", "state": "proposed"}, "open_changes": []any{}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed understanding: %v", err)
	}
}

func TestB12_ConfirmAndCorrectCaseKeepHumanTextLiteral(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID

	view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if len(view.Cases) != 1 || len(view.Sections) != 1 || view.Sections[0].RoleState != "proposed" {
		t.Fatalf("understanding view: %+v", view)
	}

	rec := do(t, h, http.MethodPost, base+"/cases/case-1/confirm", nil)
	if rec.Code != http.StatusOK || decode[server.CaseView](t, rec).State != "confirmed" {
		t.Fatalf("confirm case: got %d (%s)", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPost, base+"/cases/case-1/correct",
		map[string]any{"text": "Un muestreo de grifos reales, <tal cual> lo digo yo."})
	got := decode[server.CaseView](t, rec)
	if got.State != "corrected" || got.Text != "Un muestreo de grifos reales, <tal cual> lo digo yo." {
		t.Fatalf("correct case: %+v", got)
	}
}

func TestB12_ConfirmingRoleLeavesItConfirmedWithItsEvent(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID

	rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "recommendation"})
	if rec.Code != http.StatusOK {
		t.Fatalf("put role: got %d (%s)", rec.Code, rec.Body.String())
	}
	got := decode[server.SectionRoleView](t, rec)
	if got.Role != "recommendation" || got.RoleState != "confirmed" {
		t.Fatalf("role view: %+v", got)
	}
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == store.EventRoleChanged {
			found = true
		}
	}
	if !found {
		t.Fatal("no role_changed event recorded")
	}

	rec = do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "bogus"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid role: got %d, want 400", rec.Code)
	}
}
