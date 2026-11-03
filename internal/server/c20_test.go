package server_test

import (
	"context"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// C.20 (hallazgo 22, B-79): saves of a section less than 5 s apart are one
// edition for «Deshacer», and «Auditar» says what it costs before you press it.

func groupAPI(t *testing.T, text string) (http.Handler, *store.DB, string, *time.Duration) {
	t.Helper()
	later := new(time.Duration)
	h, db := newAPIAt(t, later)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, text, false)
	return h, db, "/api/projects/" + projectID, later
}

func TestC20_TwoSavesInARowUndoWithOneClick(t *testing.T) {
	h, db, base, later := groupAPI(t, proposalText)
	saveOn(t, h, base, "sec-1", "Primero.")
	*later += 4 * time.Second
	saveOn(t, h, base, "sec-1", "Primero y segundo.")

	got := undoRoute(t, h, base, "sec-1")
	if got["restored_version"] != float64(1) {
		t.Fatalf("undo: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != proposalText || detail["undo_available"] != false || detail["undo_reason"] != "no_edit" {
		t.Fatalf("one click goes back to before both: %v", detail)
	}
	undone := projectEvents(t, db, base, store.EventEditUndone)
	if len(undone) != 1 || undone[0]["undone_version"] != float64(3) || undone[0]["restored_version"] != float64(1) {
		t.Fatalf("one undo event: %v", undone)
	}
	if edits := projectEvents(t, db, base, store.EventEdit); len(edits) != 2 {
		t.Fatalf("history only grows, both saves stay: %v", edits)
	}
}

func TestB79_ThreeSavesWithASixSecondGapUndoOnlyTheThird(t *testing.T) {
	h, _, base, later := groupAPI(t, proposalText)
	saveOn(t, h, base, "sec-1", "Uno.")
	*later += time.Second
	saveOn(t, h, base, "sec-1", "Uno, dos.")
	*later += 6 * time.Second
	saveOn(t, h, base, "sec-1", "Uno, dos, tres.")

	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Uno, dos." || detail["undo_available"] != true {
		t.Fatalf("only the third: %v", detail)
	}
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != proposalText || detail["undo_available"] != false {
		t.Fatalf("the first two are one edition: %v", detail)
	}
}

// «a menos de 5 s»: exactly 5 s is another edition.
func TestC20_FiveSecondsApartAreTwoEditions(t *testing.T) {
	h, _, base, later := groupAPI(t, proposalText)
	saveOn(t, h, base, "sec-1", "Uno.")
	*later += 5 * time.Second
	saveOn(t, h, base, "sec-1", "Uno, dos.")
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Uno." {
		t.Fatalf("after one undo: %v", detail)
	}
}

func TestC20_TheAgentsEditInBetweenEndsTheGroup(t *testing.T) {
	h, db, base, _ := groupAPI(t, proposalText)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	projectID := base[len("/api/projects/"):]
	if applied := stepOn(t, db, projectID, replaceStep(2, "Se factura al mes.", "Se factura al año.")); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	agents := sectionDetail(t, h, base, "sec-1")["text"]
	saveOn(t, h, base, "sec-1", "Cobramos por licencia. Se factura al año. Mío.")

	undoRoute(t, h, base, "sec-1")
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != agents || detail["undo_available"] != false || detail["undo_reason"] != "agent_wrote" {
		t.Fatalf("only the save after the agent: %v", detail)
	}
}

func TestC20_AnotherSectionsSaveInBetweenEndsTheGroup(t *testing.T) {
	h, db, base, _ := groupAPI(t, proposalText)
	seedSignSection(t, db, base[len("/api/projects/"):], "Otra sección.", false)
	saveOn(t, h, base, "sec-1", "Uno.")
	saveOn(t, h, base, "sec-2", "Otra sección, cambiada.")
	saveOn(t, h, base, "sec-1", "Uno, dos.")
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Uno." {
		t.Fatalf("after one undo: %v", detail)
	}
}

// An undo ends the group too: what you write next is a new edition.
func TestC20_ASaveAfterAnUndoIsANewEdition(t *testing.T) {
	h, _, base, later := groupAPI(t, proposalText)
	saveOn(t, h, base, "sec-1", "Uno.")
	*later += 6 * time.Second
	saveOn(t, h, base, "sec-1", "Dos.")
	undoRoute(t, h, base, "sec-1")
	saveOn(t, h, base, "sec-1", "Tres.")
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Uno." || detail["undo_available"] != true {
		t.Fatalf("after undoing the new one: %v", detail)
	}
}

// B-78: the card names one edit; inside a group that edit is the edition, so
// «Fue un error · deshacer» undoes the whole group and closes the group's cards.
func TestB78_ACardOfAnEditInAGroupUndoesTheWholeGroup(t *testing.T) {
	later := new(time.Duration)
	h, db := newAPIAt(t, later)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "Porque compara igual."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	before := sectionDetail(t, h, base, "sec-1")["text"]
	saved := saveOn(t, h, base, "sec-1", "La embotellada trae 61 veces más.")
	card := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)).Cards[0]
	saveOn(t, h, base, "sec-1", "La embotellada trae 61 veces más. Sí.")

	again := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)).Cards[0]
	if again.EditVersion != int(saved["version"].(float64)) || !again.UndoAvailable || again.UndoReason != "" {
		t.Fatalf("still offered, in its group: %+v", again)
	}
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", map[string]any{"edit_version": again.EditVersion})
	if rec.Code != http.StatusOK {
		t.Fatalf("fue un error: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := db.GetObject(context.Background(), projectID, card.Label); got.State != "obsolete" {
		t.Fatalf("card: %s", got.State)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != before {
		t.Fatalf("section: %v", detail)
	}
}

// Rule 6 reads each save: an alert of any save in a group is undoable, and the
// group's undo takes it off the band.
func TestC20_AnAlertOfASaveInAGroupGoesWithTheGroup(t *testing.T) {
	h, db, base, _ := groupAPI(t, rule6Text)
	saveOn(t, h, base, "sec-1", "Corto.")
	saveOn(t, h, base, "sec-1", "Corto y algo más.")
	shown := sectionDetail(t, h, base, "sec-1")["alerts"].([]any)
	if len(shown) != 1 || shown[0].(map[string]any)["undoable"] != true {
		t.Fatalf("shown: %v", shown)
	}
	undoRoute(t, h, base, "sec-1")
	if shown := sectionDetail(t, h, base, "sec-1")["alerts"].([]any); len(shown) != 0 {
		t.Fatalf("after the undo: %v", shown)
	}
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 1 {
		t.Fatalf("the history keeps it: %v", alerts)
	}
}

// The edit that displaced the agent's text, in a group: one undo goes back to
// what your tab read, and the proposal can still be accepted as it is.
func TestC20_AGroupStartedByADisplacingEditKeepsItsProposal(t *testing.T) {
	h, db, base, _ := groupAPI(t, proposalText)
	projectID := base[len("/api/projects/"):]
	if applied := stepOn(t, db, projectID, replaceStep(1, "Cobramos por asiento.", "Cobramos por uso.")); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Cobramos por licencia."}); rec.Code != http.StatusOK {
		t.Fatalf("save: %d", rec.Code)
	}
	saveOn(t, h, base, "sec-1", "Cobramos por licencia y algo.")

	if got := undoRoute(t, h, base, "sec-1"); got["restored_version"] != float64(1) {
		t.Fatalf("undo: %v", got)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != proposalText {
		t.Fatalf("back to what your tab read: %v", detail)
	}
	if accepted := act(t, h, base, "prop-1", "accept"); accepted["state"] != "accepted" || accepted["why"] != nil {
		t.Fatalf("accept: %v", accepted)
	}
}

func seedRound(t *testing.T, db *store.DB, projectID, reason, turnID string, costs ...float64) {
	t.Helper()
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectRound, "closed", cycle.RoundBody{Kind: "judge", DelegationReason: reason, TurnID: turnID},
			store.EventInput{Type: store.EventStateChanged, Actor: "app", TurnID: turnID})
		return err
	})
	if err != nil {
		t.Fatalf("seed round: %v", err)
	}
	for _, cost := range costs {
		if err := db.RecordModelCall(ctx, store.ModelCall{ProjectID: projectID, TurnID: turnID, Role: string(provider.RoleJudge), CostUSD: cost}); err != nil {
			t.Fatalf("record call: %v", err)
		}
	}
}

func auditEstimate(t *testing.T, h http.Handler, projectID string) float64 {
	t.Helper()
	return decode[server.SummaryView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/summary", nil)).AuditEstimateUSD
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestC20_WithNoAuditsTheEstimateIsTheInitialValue(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedRound(t, db, projectID, cycle.JudgeReasonAudit, "t-empty") // an audit that cost nothing says nothing
	if got := auditEstimate(t, h, projectID); !near(got, cycle.AuditEstimateInitialUSD) {
		t.Fatalf("estimate: %v", got)
	}
}

func TestC20_TheEstimateIsTheMedianOfTheProjectsAudits(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedRound(t, db, projectID, cycle.JudgeReasonAudit, "t-1", 0.01, 0.02) // a whole audit: the sum of its turn
	seedRound(t, db, projectID, cycle.JudgeReasonAudit, "t-2", 0.90)
	seedRound(t, db, projectID, cycle.JudgeReasonAudit, "t-3", 0.05)
	seedRound(t, db, projectID, "closing_risk", "t-risk", 5) // not an audit: a round a risk brought
	if got := auditEstimate(t, h, projectID); !near(got, 0.05) {
		t.Fatalf("odd: %v", got)
	}
	seedRound(t, db, projectID, cycle.JudgeReasonAudit, "t-4", 0.07)
	if got := auditEstimate(t, h, projectID); !near(got, 0.06) {
		t.Fatalf("even, the mean of the middle two: %v", got)
	}

}

// Nothing blocks the agent: a chain writing elsewhere while you type does not
// split your edition. Only your own write elsewhere ends it.
func TestC20_AnAgentStepOnAnotherSectionDoesNotEndTheGroup(t *testing.T) {
	h, db, base, _ := groupAPI(t, proposalText)
	projectID := base[len("/api/projects/"):]
	seedSignSection(t, db, projectID, "Otra sección larga. Se factura al mes.", false)
	saveOn(t, h, base, "sec-1", "Uno.")
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-2", "base_version": 1, "anchor": "al mes", "claim_origin": "material", "operation": "replace", "text": "al año"}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	if other := sectionDetail(t, h, base, "sec-2"); other["text"] != "Otra sección larga. Se factura al año." {
		t.Fatalf("the agent's edit must apply on sec-2: %v", other)
	}
	saveOn(t, h, base, "sec-1", "Uno, dos.")

	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != proposalText || detail["undo_available"] != false {
		t.Fatalf("one click goes back to before both: %v", detail)
	}
}

// Your own undo on another section ends it, as your save there does.
func TestC20_YourUndoOnAnotherSectionEndsTheGroup(t *testing.T) {
	h, db, base, later := groupAPI(t, proposalText)
	seedSignSection(t, db, base[len("/api/projects/"):], "Otra sección.", false)
	saveOn(t, h, base, "sec-2", "Otra sección, cambiada.")
	*later += 6 * time.Second
	saveOn(t, h, base, "sec-1", "Uno.")
	undoRoute(t, h, base, "sec-2")
	saveOn(t, h, base, "sec-1", "Uno, dos.")
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Uno." {
		t.Fatalf("after one undo: %v", detail)
	}
}
