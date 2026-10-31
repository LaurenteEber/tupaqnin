package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// planUnderstanding is a hand-made understanding output with two
// sections, each with its focus and evidence_from (R-6, B-53).
const planUnderstanding = `{
  "understood": "Escribo el estado del arte.", "project_title": "Microplásticos en el agua",
  "evidence_required": {"value": true, "why": "Compara estudios publicados."},
  "criteria": [], "material_readings": [],
  "sections": [
    {"id": "new-1", "title": "Estudios incluidos", "role": "other",
     "focus": "Qué estudios cuentan como de campo.", "evidence_from": "Mason 2018, Kosuth 2018."},
    {"id": "new-2", "title": "Conclusiones", "role": "conclusion",
     "focus": "Si la embotellada trae más.", "evidence_from": "falta material"}
  ]
}`

// withPlan creates a project whose «Entendí» proposed planUnderstanding.
func withPlan(t *testing.T) (http.Handler, *store.DB, string) {
	t.Helper()
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	res, err := cycle.NewGate(db).EndUnderstanding(context.Background(), projectID, "turn-1", 1,
		json.RawMessage(planUnderstanding), nil)
	if err != nil || res.Status != cycle.StatusAccepted {
		t.Fatalf("EndUnderstanding: %+v, %v", res, err)
	}
	return h, db, "/api/projects/" + projectID
}

func projectEvents(t *testing.T, db *store.DB, base string, typ store.EventType) []map[string]any {
	t.Helper()
	events, err := db.EventsSince(context.Background(), base[len("/api/projects/"):], 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	var out []map[string]any
	for _, ev := range events {
		if ev.Type != typ {
			continue
		}
		data := map[string]any{"_actor": ev.Actor}
		_ = json.Unmarshal(ev.Data, &data)
		out = append(out, data)
	}
	return out
}

func focusOf(t *testing.T, h http.Handler, base, label string) string {
	t.Helper()
	return decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/"+label, nil)).Focus
}

// R-6: the «Entendí» brings, per section, what it concludes or
// addresses and where its evidence will come from.
func TestR6_UnderstandingCarriesFocusAndEvidenceFromPerSection(t *testing.T) {
	h, _, base := withPlan(t)
	view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if view.PlanConfirmed {
		t.Fatal("plan_confirmed before any answer")
	}
	want := []server.SectionRoleView{
		{Label: "sec-1", Title: "Estudios incluidos", Role: "other", RoleState: "proposed",
			Focus: "Qué estudios cuentan como de campo.", EvidenceFrom: "Mason 2018, Kosuth 2018."},
		{Label: "sec-2", Title: "Conclusiones", Role: "conclusion", RoleState: "proposed",
			Focus: "Si la embotellada trae más.", EvidenceFrom: "falta material"},
	}
	if len(view.Sections) != len(want) {
		t.Fatalf("sections: %+v", view.Sections)
	}
	for i, w := range want {
		if view.Sections[i] != w {
			t.Fatalf("section %d: %+v, want %+v", i, view.Sections[i], w)
		}
	}
}

// B-53: confirming copies each focus to its section with
// section_details_changed, «proposed by the agent, confirmed by you»;
// without confirming, no section has it.
func TestB53_ConfirmingThePlanCopiesEachFocusToItsSection(t *testing.T) {
	h, db, base := withPlan(t)
	for _, l := range []string{"sec-1", "sec-2"} {
		if f := focusOf(t, h, base, l); f != "" {
			t.Fatalf("%s focus before confirming: %q, want none", l, f)
		}
	}

	rec := do(t, h, http.MethodPost, base+"/plan/confirm", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm plan: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[server.UnderstandingView](t, rec); !got.PlanConfirmed {
		t.Fatalf("view after confirm: %+v", got)
	}
	if f := focusOf(t, h, base, "sec-1"); f != "Qué estudios cuentan como de campo." {
		t.Fatalf("sec-1 focus: %q", f)
	}
	if f := focusOf(t, h, base, "sec-2"); f != "Si la embotellada trae más." {
		t.Fatalf("sec-2 focus: %q", f)
	}
	if n := len(projectEvents(t, db, base, store.EventPlanConfirmed)); n != 1 {
		t.Fatalf("plan_confirmed: %d, want 1", n)
	}
	details := projectEvents(t, db, base, store.EventSectionDetailsChanged)
	if len(details) != 2 {
		t.Fatalf("section_details_changed: %v, want 2", details)
	}
	for _, d := range details {
		if d["_actor"] != "human" || d["proposed_by"] != "agent" {
			t.Fatalf("record: %v, want by you with proposed_by agent", d)
		}
	}

	// A second confirm changes nothing.
	if rec := do(t, h, http.MethodPost, base+"/plan/confirm", nil); rec.Code != http.StatusOK {
		t.Fatalf("second confirm: %d", rec.Code)
	}
	if n := len(projectEvents(t, db, base, store.EventPlanConfirmed)); n != 1 {
		t.Fatalf("plan_confirmed after a second confirm: %d, want 1", n)
	}
	if n := len(projectEvents(t, db, base, store.EventSectionDetailsChanged)); n != 2 {
		t.Fatalf("section_details_changed after a second confirm: %d, want 2", n)
	}
}

// R-6, B-53: correcting a line answers the plan; your text stays
// literal and the other lines pass as the agent proposed them.
func TestB53_CorrectingALineKeepsYourTextLiteral(t *testing.T) {
	h, db, base := withPlan(t)
	mine := "Solo estudios con <muestras reales>, no pilotos."
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1/details", map[string]any{"focus": mine, "plan": true})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("correct line: %d %s", rec.Code, rec.Body.String())
	}
	if f := focusOf(t, h, base, "sec-1"); f != mine {
		t.Fatalf("sec-1 focus: %q, want your literal text", f)
	}
	if f := focusOf(t, h, base, "sec-2"); f != "Si la embotellada trae más." {
		t.Fatalf("sec-2 focus: %q, want the agent's line", f)
	}
	if n := len(projectEvents(t, db, base, store.EventPlanConfirmed)); n != 1 {
		t.Fatalf("plan_confirmed: %d, want 1 (the first correction answers the plan)", n)
	}
	var yours bool
	for _, d := range projectEvents(t, db, base, store.EventSectionDetailsChanged) {
		if d["label"] == "sec-1" && d["proposed_by"] == nil {
			yours = true
		}
		if d["label"] == "sec-1" && d["proposed_by"] == "agent" {
			t.Fatalf("sec-1 recorded as the agent's line: %v", d)
		}
	}
	if !yours {
		t.Fatal("no section_details_changed of yours for sec-1")
	}
	view := decode[server.UnderstandingView](t, do(t, h, http.MethodGet, base+"/understanding", nil))
	if !view.PlanConfirmed || view.Sections[0].Focus != mine {
		t.Fatalf("view: %+v", view)
	}
}

// R-6: changing the mark from the card also answers the plan.
func TestR6_CorrectingTheMarkConfirmsThePlan(t *testing.T) {
	h, db, base := withPlan(t)
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1/role", map[string]any{"role": "conclusion", "plan": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("mark: %d %s", rec.Code, rec.Body.String())
	}
	if n := len(projectEvents(t, db, base, store.EventPlanConfirmed)); n != 1 {
		t.Fatalf("plan_confirmed: %d, want 1", n)
	}
	if f := focusOf(t, h, base, "sec-1"); f != "Qué estudios cuentan como de campo." {
		t.Fatalf("sec-1 focus: %q", f)
	}
}

// The previous behavior holds: editing the title or focus, or the role,
// outside the plan card (no plan flag) does not answer the plan.
func TestK17_EditingDetailsOutsideThePlanDoesNotConfirmIt(t *testing.T) {
	h, db, base := withPlan(t)
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1/details", map[string]any{"title": "Estudios"}); rec.Code != http.StatusNoContent {
		t.Fatalf("details: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-2/role", map[string]any{"role": "other"}); rec.Code != http.StatusOK {
		t.Fatalf("role: %d %s", rec.Code, rec.Body.String())
	}
	if n := len(projectEvents(t, db, base, store.EventPlanConfirmed)); n != 0 {
		t.Fatalf("plan_confirmed: %d, want 0", n)
	}
	if f := focusOf(t, h, base, "sec-2"); f != "" {
		t.Fatalf("sec-2 focus: %q, want none", f)
	}
}

// P-84 (T-4, Eber 31-oct): your edit from a tab that read sec-1 before
// the plan was confirmed is accepted, since confirming changed the focus
// and not the text; the write keeps the focus and lands on the current
// version.
func TestP84_YourEditOnAStaleTabAfterThePlanIsConfirmedIsAccepted(t *testing.T) {
	h, _, base := withPlan(t)
	before := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if rec := do(t, h, http.MethodPost, base+"/plan/confirm", nil); rec.Code != http.StatusOK {
		t.Fatalf("confirm plan: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": before.Version, "text": "Solo estudios de campo."})
	if rec.Code != http.StatusOK {
		t.Fatalf("save from the stale tab: %d %s, want 200", rec.Code, rec.Body.String())
	}
	after := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if after.Text != "Solo estudios de campo." || after.Focus != "Qué estudios cuentan como de campo." ||
		after.Version != before.Version+2 {
		t.Fatalf("sec-1 after: %+v, want your text on the confirmed version with its focus", after)
	}
}

// P-84 kept: when the text changed since the tab read it, your edit
// still gets 409 version_conflict.
func TestP84_YourEditAfterATextChangeIsStill409(t *testing.T) {
	h, _, base := withPlan(t)
	before := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": before.Version, "text": "Primera."}); rec.Code != http.StatusOK {
		t.Fatalf("first save: %d %s", rec.Code, rec.Body.String())
	}
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1",
		map[string]any{"base_version": before.Version, "text": "Desde la pestaña vieja."})
	if rec.Code != http.StatusConflict || decode[map[string]string](t, rec)["error"] != "version_conflict" {
		t.Fatalf("stale save: %d %s, want 409 version_conflict", rec.Code, rec.Body.String())
	}
}
