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

// P-27: the role lives in the section's body, so changing it makes a
// new version (every change is a version, specification §1.1). The
// editor takes that version when the text is unchanged: an edit after
// the role change, with the version the section list reports, saves.
func TestP27_ChangeRoleThenEditSavesTheTypedText(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID

	before := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))[0]
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "other"}); rec.Code != http.StatusOK {
		t.Fatalf("put role: got %d (%s)", rec.Code, rec.Body.String())
	}
	after := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))[0]
	if after.Text != before.Text {
		t.Fatalf("role change touched the text: %q → %q", before.Text, after.Text)
	}

	rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": after.Version, "text": "Texto tecleado."})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit after role change: got %d (%s), want 200", rec.Code, rec.Body.String())
	}
	saved := decode[[]server.SectionView](t, do(t, h, http.MethodGet, base+"/sections", nil))[0]
	if saved.Text != "Texto tecleado." {
		t.Fatalf("edit after role change: text %q not saved", saved.Text)
	}
}

// P-31: confirming a proposed role without changing it leaves the same
// value confirmed, with its role_changed event.
func TestP31_ConfirmProposedRoleKeepsItsValue(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID

	rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "conclusion"})
	got := decode[server.SectionRoleView](t, rec)
	if got.Role != "conclusion" || got.RoleState != "confirmed" {
		t.Fatalf("confirm proposed role: %+v", got)
	}
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	for _, ev := range events {
		if ev.Type == store.EventRoleChanged {
			return
		}
	}
	t.Fatal("no role_changed event recorded")
}

// P-30: sections are named by their title (B-18), never by an id.
func TestP30_SectionListCarriesTheTitle(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)

	list := decode[[]server.SectionView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/sections", nil))
	if len(list) != 1 || list[0].Title != "Síntesis" {
		t.Fatalf("section list: %+v, want the title Síntesis", list)
	}
}

// P-43: an understanding that ended pending for the agent (two calls
// without a valid output) is shown as failed, also after a reload: Go
// reads it from the events, not from the live stream.
func TestP43_UnderstandingThatFailedIsShownAsPendingForTheAgent(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID

	view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if view.Status != "none" {
		t.Fatalf("status before any understanding: %q, want none", view.Status)
	}

	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		for attempt, result := range []string{"", "agent_pending"} {
			data := map[string]any{"attempt": attempt + 1, "errors": []string{"/criteria: req-1 is not a criterion of this request"},
				"turn_kind": "understanding"}
			if result != "" {
				data["result"] = result
			}
			if _, err := tx.AppendEvent(store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
				AgentRole: "principal", TurnID: "turn-1", Data: data}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed output_retry: %v", err)
	}

	view = decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if view.Status != "agent_pending" {
		t.Fatalf("status after a failed understanding: %q, want agent_pending", view.Status)
	}
}
