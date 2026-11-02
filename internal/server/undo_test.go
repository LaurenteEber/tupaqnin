package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 4.2: undo and the suspicious edit (B-78, B-79, B-80), through the
// routes.

func undoRoute(t *testing.T, h http.Handler, base, label string) map[string]any {
	t.Helper()
	rec := do(t, h, http.MethodPost, base+"/sections/"+label+"/undo", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("undo %s: %d %s", label, rec.Code, rec.Body.String())
	}
	return decode[map[string]any](t, rec)
}

func saveOn(t *testing.T, h http.Handler, base, label, text string) map[string]any {
	t.Helper()
	version := int(sectionDetail(t, h, base, label)["version"].(float64))
	rec := do(t, h, http.MethodPut, base+"/sections/"+label, map[string]any{"base_version": version, "text": text})
	if rec.Code != http.StatusOK {
		t.Fatalf("save %s: %d %s", label, rec.Code, rec.Body.String())
	}
	return decode[map[string]any](t, rec)
}

func alertReasons(alert map[string]any) []string {
	var out []string
	list, _ := alert["reasons"].([]any)
	for _, r := range list {
		m, _ := r.(map[string]any)
		out = append(out, fmt.Sprintf("%v:%v", m["rule"], m["kind"]))
	}
	return out
}

func hasReason(alert map[string]any, want string) bool {
	for _, r := range alertReasons(alert) {
		if r == want {
			return true
		}
	}
	return false
}

// eventsCount is every event of the project.
func eventsCount(t *testing.T, db *store.DB, projectID string) int {
	t.Helper()
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	return len(events)
}

// «editar y deshacer deja la sección en su versión anterior con edit_undone».
func TestB79_EditAndUndoLeaveThePreviousVersionWithEditUndone(t *testing.T) {
	h, db, base, _ := proposalsAPI(t)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["undo_available"] != true || detail["undo_reason"] != "" {
		t.Fatalf("after your edit: %v", detail)
	}
	got := undoRoute(t, h, base, "sec-1")
	if got["version"] != float64(3) || got["restored_version"] != float64(1) || got["status"] != "draft" && got["status"] != "ready" {
		t.Fatalf("undo: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != proposalText || detail["version"] != float64(3) {
		t.Fatalf("the previous version must rule: %v", detail)
	}
	undone := projectEvents(t, db, base, store.EventEditUndone)
	if len(undone) != 1 || undone[0]["_actor"] != "human" || undone[0]["restored_version"] != float64(1) ||
		undone[0]["undone_version"] != float64(2) || undone[0]["section"] != "sec-1" {
		t.Fatalf("edit_undone: %v", undone)
	}
	if edits := projectEvents(t, db, base, store.EventEdit); len(edits) != 1 {
		t.Fatalf("history only grows: %v", edits)
	}
	if detail["undo_available"] != false || detail["undo_reason"] != "no_edit" {
		t.Fatalf("nothing left to undo: %v", detail)
	}
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", nil); rec.Code != http.StatusConflict ||
		decode[map[string]any](t, rec)["reason"] != "no_edit" {
		t.Fatalf("undo with nothing: %d", rec.Code)
	}
}

// B-79: pressing it again reverts the previous one, while they are yours and
// in a row.
func TestB79_PressingAgainRevertsThePreviousEditOfYours(t *testing.T) {
	h, _, base, _ := proposalsAPI(t)
	saveOn(t, h, base, "sec-1", "Primera edición mía del cobro.")
	saveOn(t, h, base, "sec-1", "Segunda edición mía del cobro.")
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Primera edición mía del cobro." || detail["undo_available"] != true {
		t.Fatalf("after one undo: %v", detail)
	}
	undoRoute(t, h, base, "sec-1")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != proposalText || detail["undo_available"] != false {
		t.Fatalf("after two: %v", detail)
	}
}

// «deshacer una edición que abrió una tarjeta R4 la deja obsolete y no cambia
// la decisión».
func TestB79_UndoingAnEditThatOpenedAnR4CardLeavesItObsoleteAndTheDecisionAlone(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "Porque compara igual."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	ctx := context.Background()
	before, _ := db.GetObject(ctx, projectID, "dec-1")
	saveOn(t, h, base, "sec-1", "La embotellada trae 61 veces más.")
	card, err := db.GetObject(ctx, projectID, "card-2")
	if err != nil || card.State != "open" {
		t.Fatalf("the R4 card: %+v, %v", card, err)
	}

	undoRoute(t, h, base, "sec-1")
	card, _ = db.GetObject(ctx, projectID, "card-2")
	if card.State != "obsolete" {
		t.Fatalf("card: %s", card.State)
	}
	after, _ := db.GetObject(ctx, projectID, "dec-1")
	if after.State != before.State || after.CurrentVersion != before.CurrentVersion {
		t.Fatalf("the decision moved: %+v, was %+v", after, before)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != "La embotellada trae 60 veces más." || len(detail["pendings"].([]any)) != 0 {
		t.Fatalf("section: %v", detail)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.Cards) != 0 {
		t.Fatalf("todo: %+v", todo)
	}
}

// «Fue un error · deshacer» en la tarjeta R4 la deja obsolete: the card says
// which section and edit, and undoing that edit closes it.
func TestB78_FueUnErrorOnTheR4CardLeavesItObsolete(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "Porque compara igual."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	saved := saveOn(t, h, base, "sec-1", "La embotellada trae 61 veces más.")
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.Cards) != 1 {
		t.Fatalf("todo: %+v", todo)
	}
	card := todo.Cards[0]
	if card.EditSection != "sec-1" || card.EditVersion != int(saved["version"].(float64)) || !card.UndoAvailable || card.UndoReason != "" {
		t.Fatalf("the card offers its undo: %+v", card)
	}
	// A later edit of yours: the card's is no longer the last one (B-79).
	saveOn(t, h, base, "sec-1", "La embotellada trae 62 veces más.")
	refused := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)).Cards[0]
	if refused.UndoAvailable || refused.UndoReason != "later_edit" {
		t.Fatalf("after a later edit: %+v", refused)
	}
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", map[string]any{"edit_version": card.EditVersion})
	if rec.Code != http.StatusConflict || decode[map[string]any](t, rec)["reason"] != "later_edit" {
		t.Fatalf("undoing the card's edit under a later one: %d", rec.Code)
	}
	undoRoute(t, h, base, "sec-1")

	rec = do(t, h, http.MethodPost, base+"/sections/sec-1/undo", map[string]any{"edit_version": card.EditVersion})
	if rec.Code != http.StatusOK {
		t.Fatalf("fue un error: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := db.GetObject(context.Background(), projectID, card.Label)
	if got.State != "obsolete" {
		t.Fatalf("card: %s", got.State)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "La embotellada trae 60 veces más." {
		t.Fatalf("section: %v", detail)
	}
}

// «tras deshacer, la propuesta desplazada (4.1) sigue abierta y se puede
// aceptar»: the undo goes back to the text your tab read, the agent's text
// stays in its proposal, and accepting brings back what it carried.
func TestB79_AfterUndoTheDisplacedProposalStaysOpenAndCanBeAccepted(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{
			"kind": "file", "filename": "tarifa.md", "text": "La tarifa dice que el uso manda en el cobro."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		return tx.RecordDelivery(store.Delivery{Via: store.ViaRead, ObjectID: src.ID, Version: src.CurrentVersion})
	})
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Cobramos por asiento.", "claim_origin": "material", "operation": "replace", "text": "Cobramos 3 veces por uso."}],
	  "new_decisions": [{"id": "new-1", "statement": "Cobrar por uso.", "instead_of": "Por asiento.", "would_choose_if": "Hubiera asientos.", "assumption": false, "used_in_sections": ["sec-1"]}],
	  "figure_links": [{"figure_anchor": "3 veces", "decision_id": "new-1"}],
	  "citations": [{"claim_anchor": "por uso", "source_id": "src-1", "passage": "el uso manda"}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	agents := sectionDetail(t, h, base, "sec-1")["text"].(string)
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Cobramos por licencia."})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d", rec.Code)
	}

	got := undoRoute(t, h, base, "sec-1")
	if got["restored_version"] != float64(1) {
		t.Fatalf("the undo goes back to the version your tab read: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != proposalText || len(detail["figures"].([]any)) != 0 || len(detail["citations"].([]any)) != 0 {
		t.Fatalf("after the undo: %v", detail)
	}
	list := listProposals(t, h, base, "?section=sec-1")
	if len(list) != 1 || list[0]["state"] != "open" || list[0]["text"] != agents {
		t.Fatalf("the proposal stays open: %v", list)
	}

	accepted := act(t, h, base, "prop-1", "accept")
	if accepted["state"] != "accepted" || accepted["why"] != nil {
		t.Fatalf("accept: %v", accepted)
	}
	detail = sectionDetail(t, h, base, "sec-1")
	if detail["text"] != agents || len(detail["figures"].([]any)) != 1 || len(detail["citations"].([]any)) != 1 {
		t.Fatalf("after accepting: %v", detail)
	}
	edits := projectEvents(t, db, base, store.EventEdit)
	last := edits[len(edits)-1]
	if last["_actor"] != "agent" || last["accepted_by"] != "human" {
		t.Fatalf("edit: %v", last)
	}
	// P-115: your edit was undone, so none is left to name.
	if detail["undo_available"] != false || detail["undo_reason"] != "no_edit" {
		t.Fatalf("undo is off with its reason: %v", detail)
	}
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", nil); rec.Code != http.StatusConflict ||
		decode[map[string]any](t, rec)["reason"] != "no_edit" {
		t.Fatalf("undo after accepting: %d", rec.Code)
	}
}

// B-73 d still rules a section that moved after your undo: an edit of yours
// on top asks for a new step.
func TestB79_AfterUndoAndAnotherEditAcceptingAsksForANewStep(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if applied := stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año.")); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	saveText(t, h, base, 1, "Mío.")
	undoRoute(t, h, base, "sec-1")
	saveOn(t, h, base, "sec-1", "Mío otra vez, después de deshacer.")
	if got := act(t, h, base, "prop-1", "accept"); got["state"] != "discarded" || got["why"] != "section_changed" {
		t.Fatalf("accept: %v", got)
	}
}

// «editar una sección firmada y deshacer la deja signed sin firmar de nuevo».
func TestB78_EditingASignedSectionAndUndoingLeavesItSigned(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	signoffsBefore := len(projectEvents(t, db, base, store.EventSignoffCase))
	saveOn(t, h, base, "sec-1", "Otro texto, mío, sobre lo firmado.")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["status"] != "changed_since_signoff" {
		t.Fatalf("after your edit: %v", detail)
	}
	got := undoRoute(t, h, base, "sec-1")
	if got["status"] != "signed" {
		t.Fatalf("undo: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "signed" || detail["text"] != proposalText {
		t.Fatalf("after undoing: %v", detail)
	}
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if sec.State != "signed" {
		t.Fatalf("stored state: %s", sec.State)
	}
	if n := len(projectEvents(t, db, base, store.EventSignoffCase)); n != signoffsBefore {
		t.Fatalf("no new signing: %d", n)
	}
	moved := projectEvents(t, db, base, store.EventStateChanged)
	last := moved[len(moved)-1]
	if last["to"] != "signed" || last["from"] != "changed_since_signoff" || last["via"] != "edit_undone" {
		t.Fatalf("the move: %v", last)
	}
}

// B-78: with something open it stays in «cambió», whatever the fingerprint.
func TestB78_WithAnOpenChangeTheUndoLeavesItChanged(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	saveOn(t, h, base, "sec-1", "Otro texto, mío, sobre lo firmado.")
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["open_changes"] = []any{map[string]any{"label": "chg-1", "kind": "other", "what": "x"}}
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventUsageChange, Actor: "app"})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
}

// An edit made before signing is still yours: undoing it changes what you
// signed, so the section moves to «cambió», like any edit of yours (estados §1).
func TestB79_UndoingAnEditFromBeforeTheSigningMovesItToChanged(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveOn(t, h, base, "sec-1", "Lo que firmé: un texto mío.")
	signSection(t, db, projectID)
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
}

// «con un paso del agente después de tu edición, «Deshacer» se niega con su
// razón».
func TestB79_WithAnAgentStepAfterYourEditUndoRefusesWithItsReason(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	if applied := stepOn(t, db, projectID, replaceStep(2, "Se factura al mes.", "Se factura al año.")); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["undo_available"] != false || detail["undo_reason"] != "agent_wrote" {
		t.Fatalf("after the agent's step: %v", detail)
	}
	before := eventsCount(t, db, projectID)
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("undo: %d", rec.Code)
	}
	body := decode[map[string]any](t, rec)
	if body["error"] != "undo_unavailable" || body["reason"] != "agent_wrote" {
		t.Fatalf("refusal: %v", body)
	}
	if n := eventsCount(t, db, projectID); n != before {
		t.Fatalf("a refusal writes nothing: %d events, was %d", n, before)
	}
	if after := sectionDetail(t, h, base, "sec-1"); after["text"] != detail["text"] || after["version"] != detail["version"] {
		t.Fatalf("the section moved: %v", after)
	}
}

// The undo passes rules 1 to 5 like any edit (flujos §1): the text it brings
// back names a project label, and rule 5 finds it again.
func TestB79_TheRestoredVersionPassesTheRules(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveOn(t, h, base, "sec-1", "Un texto mío sin cifras y más largo que el de antes.")
	ctx := context.Background()
	// The version your edit left behind names a project label, which rule 5
	// only finds when it is read again: the undo reads it.
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "S.", "instead_of": "I.", "origin": "agent"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	saveOn(t, h, base, "sec-1", "Ver dec-1: este texto nombra una etiqueta interna del proyecto.")
	saveOn(t, h, base, "sec-1", "Otro texto mío, ya sin etiquetas.")
	undoRoute(t, h, base, "sec-1")
	pendings := sectionDetail(t, h, base, "sec-1")["pendings"].([]any)
	if len(pendings) != 1 || pendings[0].(map[string]any)["kind"] != "internal_codes" {
		t.Fatalf("pendings after the undo: %v", pendings)
	}
}

// B-36: a restored version enters the batch for the checker with the same
// shape as your edit.
func TestB36_ARestoredVersionEntersTheBatchOfYourEdits(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveOn(t, h, base, "sec-1", "Un texto mío, otro cualquiera.")
	ctx := context.Background()
	// The checker judged that edit.
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "app", Data: map[string]any{"sections": []string{"sec-1"}}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := checks.YourEditsToCheck(ctx, db, projectID)
	if err != nil || len(batch.Sections) != 0 {
		t.Fatalf("batch before: %+v, %v", batch, err)
	}
	undoRoute(t, h, base, "sec-1")
	batch, err = checks.YourEditsToCheck(ctx, db, projectID)
	if err != nil || len(batch.Passages) != 1 || batch.Passages[0].Text != proposalText || batch.Kind != checks.YourEdit {
		t.Fatalf("batch after: %+v, %v", batch, err)
	}
}

// B-73: an undo counts as your edit for a step on an earlier version: the
// step's text goes to a proposal, it is not applied to what you restored.
func TestB73a_AStepOnAVersionBeforeYourUndoIsAProposal(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	undoRoute(t, h, base, "sec-1")
	if applied := stepOn(t, db, projectID, replaceStep(2, "Se factura al mes.", "Se factura al año.")); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != proposalText {
		t.Fatalf("your undo rules: %v", detail)
	}
	list := listProposals(t, h, base, "?section=sec-1")
	if len(list) != 1 || list[0]["origin"] != "your_edit" {
		t.Fatalf("proposal: %v", list)
	}
}

// 4.4: an undo that closes a signoff cancels it and tells the runner.
func TestK16_AnUndoOnASectionInSignoffCancelsItAndSaysSo(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	spy := &closeSpy{stubStarter: &stubStarter{}}
	h := withVisitor(t, db, server.NewWithAPI(nil, server.NewHub(), db, spy))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, proposalText, false)
	saveOn(t, h, base, "sec-1", "Un texto mío antes de pedir firmar.")
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), projectID, "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	undoRoute(t, h, base, "sec-1")
	if view := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/signoffs/"+open.Label, nil)); view["phase"] != "cancelled" {
		t.Fatalf("the signoff: %v", view)
	}
	if spy.closed != 1 {
		t.Fatalf("closed told %d times", spy.closed)
	}
}

// A second tab is read-only, for an undo as for a save (specification §2).
func TestB79_ASecondTabCannotUndo(t *testing.T) {
	h, _, base, _ := proposalsAPI(t)
	if rec := do(t, h, http.MethodPost, base+"/editor", map[string]any{"tab_id": "one"}); rec.Code != http.StatusOK {
		t.Fatalf("editor: %d", rec.Code)
	}
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/undo", map[string]any{"tab_id": "two"})
	if rec.Code != http.StatusConflict || decode[map[string]any](t, rec)["error"] != "read_only_tab" {
		t.Fatalf("undo: %d", rec.Code)
	}
}

func TestB79_UndoOnWhatIsNotASectionIsNotFound(t *testing.T) {
	h, _, base, _ := proposalsAPI(t)
	if rec := do(t, h, http.MethodPost, base+"/sections/sec-9/undo", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("undo: %d", rec.Code)
	}
}

// The six cases of the row's «borrar un párrafo…» sentence, each with its own
// assertion.

const rule6Text = "Primer párrafo: cobramos por asiento a todas las empresas del catálogo.\n\n" +
	"Segundo párrafo: el descuento anual se negocia caso por caso con cada cliente.\n\n" +
	"Tercer párrafo: la factura sale cada mes y se paga a treinta días de plazo."

func rule6API(t *testing.T) (http.Handler, *store.DB, string, string) {
	t.Helper()
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, rule6Text, false)
	return h, db, "/api/projects/" + projectID, projectID
}

// Regla 1: borrar un párrafo entero.
func TestB80_DeletingAWholeParagraphLeavesAnAlert(t *testing.T) {
	h, db, base, _ := rule6API(t)
	text := strings.Replace(rule6Text, "Segundo párrafo: el descuento anual se negocia caso por caso con cada cliente.\n\n", "", 1)
	saved := saveOn(t, h, base, "sec-1", text)
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "1:paragraph_deleted") {
		t.Fatalf("alerts: %v", alerts)
	}
	if alerts[0]["_actor"] != "app" || alerts[0]["section"] != "sec-1" || alerts[0]["version"] != saved["version"] {
		t.Fatalf("alert: %v", alerts[0])
	}
	// It never blocks: your edit rules, and the alert stays unanswered.
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != text {
		t.Fatalf("your edit rules: %v", detail)
	}
	shown := detail["alerts"].([]any)
	if len(shown) != 1 {
		t.Fatalf("shown: %v", detail["alerts"])
	}
	a := shown[0].(map[string]any)
	if a["version"] != saved["version"] || a["undoable"] != true || a["seq"] == nil {
		t.Fatalf("alert view: %v", a)
	}
}

// Regla 1: quitar más de la mitad del texto, sin borrar ningún bloque.
func TestB80_RemovingMoreThanHalfLeavesAnAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	long := "Este texto es de un solo párrafo y bastante largo, porque quiero quitar más de la mitad sin borrar el bloque."
	seedSignSection(t, db, projectID, long, false)
	base := "/api/projects/" + projectID
	saveOn(t, h, base, "sec-1", "Este texto es de un solo párrafo.")
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "1:over_half") || hasReason(alerts[0], "1:paragraph_deleted") {
		t.Fatalf("alerts: %v", alerts)
	}
}

// Regla 2: quitar el ancla de una cifra vinculada.
func TestB80_RemovingTheAnchorOfALinkedFigureLeavesAnAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	seedCriticalDecision(t, db, projectID)
	seedCoveredFigure(t, db, projectID)
	base := "/api/projects/" + projectID
	saveOn(t, h, base, "sec-1", "La embotellada trae 61 veces más.")
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "2:figure_anchor") {
		t.Fatalf("alerts: %v", alerts)
	}
	reason := alerts[0]["reasons"].([]any)[0].(map[string]any)
	if reason["about"] != "60 veces" || reason["decision"] != "dec-1" || reason["statement"] == "" {
		t.Fatalf("reason: %v", reason)
	}
}

// Regla 2: quitar el ancla de una cita vinculada.
func TestB80_RemovingTheAnchorOfALinkedCitationLeavesAnAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, "El agua trae microplásticos, según el estudio.", false)
	base := "/api/projects/" + projectID
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, _ := tx.GetObject(ctx, projectID, "sec-1")
		src, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "f.md", "text": "El agua trae microplásticos."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		c, err := tx.CreateObject(store.ObjectCitation, "", map[string]any{"section_id": sec.ID, "anchor": "microplásticos", "passage": "trae microplásticos"},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeComesFrom, c.ID, src.ID, "microplásticos", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	saveOn(t, h, base, "sec-1", "El agua trae plásticos, según el estudio.")
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "2:citation_anchor") {
		t.Fatalf("alerts: %v", alerts)
	}
}

// Regla 2: un bloque cuya historia (B-10) muestra una decisión.
func TestB80_RemovingABlockWhoseHistoryShowsADecisionNamesIt(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Se factura al mes.", "claim_origin": "material", "operation": "insert_after", "text": "\n\nEl descuento se negocia por cliente."}],
	  "new_decisions": [{"id": "new-1", "statement": "Negociar el descuento.", "instead_of": "Tarifa fija.", "would_choose_if": "Hubiera pocos clientes.", "assumption": false, "used_in_sections": ["sec-1"]}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	saveOn(t, h, base, "sec-1", proposalText)
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "2:decision_text") {
		t.Fatalf("alerts: %v", alerts)
	}
	var found bool
	for _, r := range alerts[0]["reasons"].([]any) {
		m := r.(map[string]any)
		if m["kind"] == "decision_text" && m["statement"] == "Negociar el descuento." {
			found = true
		}
	}
	if !found {
		t.Fatalf("the reason names the decision: %v", alerts[0])
	}
}

// Regla 2: 20 caracteres o más quitados de una sección firmada; 19, no.
func TestB80_RemovingTwentyCharactersOfASignedSectionLeavesAnAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, proposalText+" Una línea más larga para que veinte no sea la mitad del texto firmado.", false)
	base := "/api/projects/" + projectID
	signSection(t, db, projectID)
	text := sectionDetail(t, h, base, "sec-1")["text"].(string)
	saveOn(t, h, base, "sec-1", strings.Replace(text, " para que veinte no", " no", 1)) // 16 characters
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("16 on a signed section: %v", alerts)
	}
	undoRoute(t, h, base, "sec-1")
	saveOn(t, h, base, "sec-1", strings.Replace(text, " más larga para que veinte", "", 1)) // 26 characters
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "2:signed_text") {
		t.Fatalf("alerts: %v", alerts)
	}
}

// Regla 3: quitar 20 caracteres de otra sección a los 5 min de la anterior; a
// los 11 min, no.
func TestB80_RemovingTwentyCharactersOfAnotherSectionFiveMinutesLaterLeavesAnAlert(t *testing.T) {
	later := time.Duration(0)
	h, db := newAPIAt(t, &later)
	projectID := newProject(t, h, nil)
	long := "Una sección bastante larga, con mucho texto que no se va a quitar en esta prueba de la regla tres, y una frase de veinte caracteres aquí."
	seedSignSection(t, db, projectID, long, false)
	seedSignSection(t, db, projectID, long, false)
	base := "/api/projects/" + projectID
	saveOn(t, h, base, "sec-1", long+" Añadido.")

	later = 11 * time.Minute
	saveOn(t, h, base, "sec-2", strings.Replace(long, ", y una frase de veinte caracteres aquí", "", 1))
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("at 11 min: %v", alerts)
	}
	undoRoute(t, h, base, "sec-2")

	saveOn(t, h, base, "sec-1", long+" Añadido otra vez.")
	later = 5 * time.Minute
	saveOn(t, h, base, "sec-2", strings.Replace(long, ", y una frase de veinte caracteres aquí", "", 1))
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "3:other_section") {
		t.Fatalf("alerts: %v", alerts)
	}
	if reason := alerts[0]["reasons"].([]any)[0].(map[string]any); reason["about"] != "Regla de comparación" {
		t.Fatalf("reason: %v", reason)
	}
}

// Regla 4: pegar 80 caracteres que ya están en una fuente.
func TestB80_PastingEightyCharactersAlreadyInASourceLeavesAnAlert(t *testing.T) {
	h, db, base, projectID := rule6API(t)
	passage := "El estudio de la cuenca comparó cuarenta muestras de agua embotellada con la del grifo durante dos años."
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "cuenca.md", "text": "Antes.\n\n" + passage + "\n\nDespués."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	saveOn(t, h, base, "sec-1", rule6Text+"\n\nComo dice: "+passage+" Fin.")
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "4:pasted") {
		t.Fatalf("alerts: %v", alerts)
	}
	if reason := alerts[0]["reasons"].([]any)[0].(map[string]any); reason["about"] != "cuenca.md" {
		t.Fatalf("reason: %v", reason)
	}
}

// Regla 4: también lo que ya está en otra sección.
func TestB80_PastingEightyCharactersAlreadyInAnotherSectionLeavesAnAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	passage := "El estudio de la cuenca comparó cuarenta muestras de agua embotellada con la del grifo durante dos años."
	seedSignSection(t, db, projectID, rule6Text, false)
	seedSignSection(t, db, projectID, passage, false)
	base := "/api/projects/" + projectID
	saveOn(t, h, base, "sec-1", rule6Text+"\n\n"+passage)
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "4:pasted") {
		t.Fatalf("alerts: %v", alerts)
	}
}

// «una corrección de una coma… no».
func TestB80_FixingACommaLeavesNoAlert(t *testing.T) {
	h, db, base, _ := rule6API(t)
	saveOn(t, h, base, "sec-1", strings.Replace(rule6Text, "Primer párrafo: cobramos por asiento a todas", "Primer párrafo: cobramos por asiento, a todas", 1))
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("alerts: %v", alerts)
	}
	if shown := sectionDetail(t, h, base, "sec-1")["alerts"].([]any); len(shown) != 0 {
		t.Fatalf("shown: %v", shown)
	}
}

// «…o editar otra sección sin quitar texto, no».
func TestB80_EditingAnotherSectionWithoutRemovingTextLeavesNoAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, rule6Text, false)
	seedSignSection(t, db, projectID, rule6Text+" Otra.", false)
	base := "/api/projects/" + projectID
	saveOn(t, h, base, "sec-1", rule6Text+" Un añadido.")
	saveOn(t, h, base, "sec-2", rule6Text+" Otra. Un añadido.")
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("alerts: %v", alerts)
	}
}

// Rule 6 is on your edits only (B-80): an undo and an agent's step raise none.
func TestB80_AnUndoAndAStepLeaveNoAlert(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	// Your edit only adds; undoing it takes that much out.
	saveOn(t, h, base, "sec-1", proposalText+" Y una frase bastante larga que añado para luego quitarla con el botón de deshacer.")
	undoRoute(t, h, base, "sec-1")
	if applied := stepOn(t, db, projectID, `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 3, "anchor": "Cobramos por asiento. Se factura al mes.", "claim_origin": "material", "operation": "replace", "text": "Corto."}]}`); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("alerts: %v", alerts)
	}
}

// The interface: POST /alerts/{n}/answer adds another alert with answer_to and
// deliberate; history only grows.
func TestB80_AnsweringAnAlertAddsAnotherWithAnswerTo(t *testing.T) {
	h, db, base, projectID := rule6API(t)
	saveOn(t, h, base, "sec-1", "Corto.")
	shown := sectionDetail(t, h, base, "sec-1")["alerts"].([]any)
	if len(shown) != 1 {
		t.Fatalf("shown: %v", shown)
	}
	seq := int(shown[0].(map[string]any)["seq"].(float64))
	rec := do(t, h, http.MethodPost, fmt.Sprintf("%s/alerts/%d/answer", base, seq), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("answer: %d %s", rec.Code, rec.Body.String())
	}
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 2 || alerts[1]["_actor"] != "human" || alerts[1]["answer_to"] != float64(seq) || alerts[1]["deliberate"] != true {
		t.Fatalf("alerts: %v", alerts)
	}
	if after := sectionDetail(t, h, base, "sec-1"); len(after["alerts"].([]any)) != 0 || after["text"] != "Corto." {
		t.Fatalf("after answering: %v", after)
	}
	// The answer itself is not an alert to answer, and an alert answers once.
	before := eventsCount(t, db, projectID)
	if rec := do(t, h, http.MethodPost, fmt.Sprintf("%s/alerts/%d/answer", base, before), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("answering the answer: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, fmt.Sprintf("%s/alerts/%d/answer", base, seq), nil); rec.Code != http.StatusConflict {
		t.Fatalf("answering twice: %d", rec.Code)
	}
	if n := eventsCount(t, db, projectID); n != before {
		t.Fatalf("a refused answer writes nothing: %d, was %d", n, before)
	}
}

// B-80: an undone edit's alert leaves the band: there is nothing left to
// answer, and it stays in the history.
func TestB80_UndoingTheEditTakesItsAlertOffTheBand(t *testing.T) {
	h, db, base, _ := rule6API(t)
	saveOn(t, h, base, "sec-1", "Corto.")
	undoRoute(t, h, base, "sec-1")
	if shown := sectionDetail(t, h, base, "sec-1")["alerts"].([]any); len(shown) != 0 {
		t.Fatalf("shown: %v", shown)
	}
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 1 {
		t.Fatalf("alerts: %v", alerts)
	}
}

// The alert's «Deshacer» says when it no longer undoes that edit (B-79).
func TestB80_AnAlertUnderALaterEditIsNotUndoable(t *testing.T) {
	h, _, base, _ := rule6API(t)
	saveOn(t, h, base, "sec-1", "Corto.")
	saveOn(t, h, base, "sec-1", "Corto y algo más.")
	a := sectionDetail(t, h, base, "sec-1")["alerts"].([]any)[0].(map[string]any)
	if a["undoable"] != false || a["undo_reason"] != "later_edit" {
		t.Fatalf("alert: %v", a)
	}
}

// `n` is an event's seq: another project's seq or a seq that is not an alert
// answers as not found, and writes nothing.
func TestB80_AnAlertOfAnotherProjectOrNotAnAlertIsNotFound(t *testing.T) {
	h, db, base, projectID := rule6API(t)
	saveOn(t, h, base, "sec-1", "Corto.")
	seq := int(sectionDetail(t, h, base, "sec-1")["alerts"].([]any)[0].(map[string]any)["seq"].(float64))

	// Another visitor's project: a visitor without an account has one.
	h2 := anotherVisitor(t, db, h)
	other := newProject(t, h2, nil)
	otherBase := "/api/projects/" + other
	seedSignSection(t, db, other, "Otro.", false)
	otherBefore := eventsCount(t, db, other)
	if otherBefore >= seq {
		t.Fatalf("the other project must not reach seq %d: %d", seq, otherBefore)
	}
	if rec := do(t, h2, http.MethodPost, fmt.Sprintf("%s/alerts/%d/answer", otherBase, seq), nil); rec.Code != http.StatusNotFound ||
		decode[map[string]any](t, rec)["error"] != "alert_not_found" {
		t.Fatalf("another project's seq: %d", rec.Code)
	}
	if n := eventsCount(t, db, other); n != otherBefore {
		t.Fatalf("it wrote: %d, was %d", n, otherBefore)
	}
	before := eventsCount(t, db, projectID)
	for _, n := range []string{"1", "abc", "0", "-3", "99999"} {
		if rec := do(t, h, http.MethodPost, base+"/alerts/"+n+"/answer", nil); rec.Code != http.StatusNotFound {
			t.Fatalf("seq %s: %d", n, rec.Code)
		}
	}
	if n := eventsCount(t, db, projectID); n != before {
		t.Fatalf("it wrote: %d, was %d", n, before)
	}
}

// Existing behaviour sharing the code: a normal edit stays a plain edit event
// with no data of its own.
func TestB79_ANormalEditIsStillAnEditWithoutData(t *testing.T) {
	h, db, base, projectID := rule6API(t)
	saveOn(t, h, base, "sec-1", rule6Text+" Un añadido.")
	events, err := db.EventsSince(context.Background(), projectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.Type != store.EventEdit || last.Actor != "human" || len(last.Data) != 0 {
		t.Fatalf("event: %+v", last)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["version"] != float64(2) {
		t.Fatalf("detail: %v", detail)
	}
}

// Existing behaviour: a stale save of yours over your own edit stays 409, and
// over your undo too.
func TestP84_AStaleSaveOverYourOwnEditOrUndoIsStillAVersionConflict(t *testing.T) {
	h, _, base, _ := proposalsAPI(t)
	saveText(t, h, base, 1, "Primera edición mía del cobro.")
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Otra."}); rec.Code != http.StatusConflict {
		t.Fatalf("over your edit: %d", rec.Code)
	}
	saveOn(t, h, base, "sec-1", "Segunda edición mía del cobro.")
	undoRoute(t, h, base, "sec-1")
	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": "Otra."}); rec.Code != http.StatusConflict {
		t.Fatalf("over your undo: %d", rec.Code)
	}
}

// Existing behaviour: your edit on a signed section still moves it.
func TestB78_AnEditOnASignedSectionStillMovesItToChanged(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	saved := saveOn(t, h, base, "sec-1", proposalText+" Un añadido.")
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "changed_since_signoff" || detail["version"] != saved["version"] {
		t.Fatalf("detail: %v", detail)
	}
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	var b map[string]any
	_ = json.Unmarshal(sec.Body, &b)
	if fingerprint.Section(b["text"].(string)) == fingerprint.Section(proposalText) {
		t.Fatalf("the edit must rule: %v", b)
	}
}

// Existing behaviour: accepting a proposal with no undo applies it, as 4.1.
func TestB75_AcceptingWithNoUndoStillApplies(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if applied := stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año.")); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	saveText(t, h, base, 1, "Mío.")
	got := act(t, h, base, "prop-1", "accept")
	if got["state"] != "accepted" {
		t.Fatalf("accept: %v", got)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Cobramos por asiento. Se factura al año." {
		t.Fatalf("detail: %v", detail)
	}
}

// A signed section with a source of its own, and the change the source's new
// version enters.

func chgOf(t *testing.T, db *store.DB, projectID, label string) string {
	t.Helper()
	sec, err := db.GetObject(context.Background(), projectID, label)
	if err != nil {
		t.Fatal(err)
	}
	changes := graph.OpenChanges(sec)
	if len(changes) == 0 {
		t.Fatalf("%s has no open change", label)
	}
	return changes[0].Label
}

func signedWithSource(t *testing.T) (http.Handler, *store.DB, string, string) {
	t.Helper()
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := uploadReplacing(t, h, base, "", "calc.txt", []byte("Resultado uno.")); rec.Code >= 300 {
		t.Fatalf("upload: %d", rec.Code)
	}
	seedSectionsUsing(t, db, projectID)
	signSection(t, db, projectID)
	return h, db, base, projectID
}

func replaceTheSource(t *testing.T, h http.Handler, base string) {
	t.Helper()
	if rec := uploadReplacing(t, h, base, "src-1", "calc.txt", []byte("Resultado dos, otro.")); rec.Code >= 300 {
		t.Fatalf("replace: %d", rec.Code)
	}
}

// B-78 with spec §4: a decision that became critical after the signing. The
// signature holds no contrast of it, so an undo does not sign it back.
func TestB78_S4_ADecisionCriticalAfterTheSigningKeepsItChangedAfterUndo(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := uploadReplacing(t, h, base, "", "calc.txt", []byte("Resultado uno.")); rec.Code >= 300 {
		t.Fatalf("upload: %d", rec.Code)
	}
	seedSectionsUsing(t, db, projectID)
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, _ := tx.GetObject(ctx, projectID, "sec-1")
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{"statement": "Cobrar por asiento.", "instead_of": "Por uso.",
			"would_choose_if": "Hubiera uso medido.", "origin": "agent"}, store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	signSection(t, db, projectID)
	if rec := do(t, h, http.MethodPut, base+"/decisions/dec-1/critical", map[string]any{"marked": true}); rec.Code != http.StatusNoContent {
		t.Fatalf("mark: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, base+"/cards/card-1/answer", map[string]any{"option": "keep", "reason": "Porque compara igual en todos los casos."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	saveOn(t, h, base, "sec-1", "Otro texto, mío.")
	got := undoRoute(t, h, base, "sec-1")
	if got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["status"] != "changed_since_signoff" || detail["text"] != "Texto de Síntesis." {
		t.Fatalf("detail: %v", detail)
	}
}

// B-78 with spec §4: another source fingerprint, its change closed as reviewed.
func TestB78_S4_ASourceThatChangedKeepsItChangedAfterUndo(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	replaceTheSource(t, h, base)
	saveOn(t, h, base, "sec-1", "Otro texto, mío.")
	rec := do(t, h, http.MethodPost, base+"/changes/"+chgOf(t, db, projectID, "sec-1")+"/close",
		map[string]any{"result": "reviewed_unchanged", "reason": "Porque el resultado no toca esta sección."})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("close: %d %s", rec.Code, rec.Body.String())
	}
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
}

// The counterpart: the same components as the signature, source included, sign
// it back.
func TestB78_S4_WithTheSameSourceTheUndoLeavesItSigned(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	saveOn(t, h, base, "sec-1", "Otro texto, mío.")
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "signed" {
		t.Fatalf("undo: %v", got)
	}
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if sec.State != "signed" {
		t.Fatalf("state: %s", sec.State)
	}
}

// With its change still open it stays changed.
func TestB78_A1_WithTheSourcesChangeOpenTheUndoLeavesItChanged(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	replaceTheSource(t, h, base)
	saveOn(t, h, base, "sec-1", "Otro.")
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
	if chgOf(t, db, projectID, "sec-1") == "" {
		t.Fatal("the change stays open")
	}
}

// A-1: «Actualizado» rested on your edit: undoing it reopens the change.
func TestA1_UndoingTheEditThatClosedAChangeUpdatedReopensIt(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	replaceTheSource(t, h, base)
	saveOn(t, h, base, "sec-1", "Texto actualizado con el resultado dos.")
	change := chgOf(t, db, projectID, "sec-1")
	if rec := do(t, h, http.MethodPost, base+"/changes/"+change+"/close", map[string]any{"result": "updated"}); rec.Code != http.StatusNoContent {
		t.Fatalf("close: %d %s", rec.Code, rec.Body.String())
	}
	got := undoRoute(t, h, base, "sec-1")
	if got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
	if chgOf(t, db, projectID, "sec-1") != change {
		t.Fatalf("the change is not back: %v", sectionDetail(t, h, base, "sec-1"))
	}
	usage := projectEvents(t, db, base, store.EventUsageChange)
	last := usage[len(usage)-1]
	if last["kind"] != "entered" || last["change"] != change || last["why"] != "edit_undone" || last["_actor"] != "app" {
		t.Fatalf("usage_change: %v", last)
	}
	// It takes a new edit to close it as updated again.
	if rec := do(t, h, http.MethodPost, base+"/changes/"+change+"/close", map[string]any{"result": "updated"}); rec.Code != http.StatusConflict {
		t.Fatalf("close without an edit: %d", rec.Code)
	}
}

// A-1: an earlier edit that still stands keeps the close.
func TestA1_AnEditThatStandsKeepsTheCloseUpdatedAfterAnUndo(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	replaceTheSource(t, h, base)
	saveOn(t, h, base, "sec-1", "Texto actualizado con el resultado dos.")
	if rec := do(t, h, http.MethodPost, base+"/changes/"+chgOf(t, db, projectID, "sec-1")+"/close", map[string]any{"result": "updated"}); rec.Code != http.StatusNoContent {
		t.Fatalf("close: %d", rec.Code)
	}
	saveOn(t, h, base, "sec-1", "Texto actualizado otra vez con el resultado dos.")
	undoRoute(t, h, base, "sec-1")
	sec, _ := db.GetObject(context.Background(), projectID, "sec-1")
	if open := graph.OpenChanges(sec); len(open) != 0 {
		t.Fatalf("open: %v", open)
	}
}

// A-1: «Actualizado» needs an edit that still stands: 409 section_not_edited
// once it was undone, and no signature comes back from it.
func TestA1_UpdatedAfterYourEditWasUndoneIsRefused(t *testing.T) {
	h, db, base, projectID := signedWithSource(t)
	replaceTheSource(t, h, base)
	saveOn(t, h, base, "sec-1", "Texto actualizado.")
	undoRoute(t, h, base, "sec-1")
	before := eventsCount(t, db, projectID)
	rec := do(t, h, http.MethodPost, base+"/changes/"+chgOf(t, db, projectID, "sec-1")+"/close", map[string]any{"result": "updated"})
	if rec.Code != http.StatusConflict || decode[map[string]any](t, rec)["error"] != "section_not_edited" {
		t.Fatalf("close: %d", rec.Code)
	}
	if n := eventsCount(t, db, projectID); n != before {
		t.Fatalf("a refusal writes nothing: %d, was %d", n, before)
	}
	// The counterpart: a new edit that stands closes it.
	saveOn(t, h, base, "sec-1", "Texto actualizado de verdad.")
	if rec := do(t, h, http.MethodPost, base+"/changes/"+chgOf(t, db, projectID, "sec-1")+"/close", map[string]any{"result": "updated"}); rec.Code != http.StatusNoContent {
		t.Fatalf("close after a new edit: %d %s", rec.Code, rec.Body.String())
	}
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo: %v", got)
	}
}

// E-4: a section «escrita con lo anterior», closed as updated by an edit you
// undo, has the old text again: it does not show ready.
func TestE4_AStaleSectionClosedUpdatedAndUndoneIsStaleAgain(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := uploadReplacing(t, h, base, "", "calc.txt", []byte("Resultado uno.")); rec.Code >= 300 {
		t.Fatalf("upload: %d", rec.Code)
	}
	seedSectionsUsing(t, db, projectID)
	replaceTheSource(t, h, base)
	if s := sectionDetail(t, h, base, "sec-1")["status"]; s != "stale" {
		t.Fatalf("status: %v", s)
	}
	saveOn(t, h, base, "sec-1", "Texto con lo nuevo.")
	change := chgOf(t, db, projectID, "sec-1")
	if rec := do(t, h, http.MethodPost, base+"/changes/"+change+"/close", map[string]any{"result": "updated"}); rec.Code != http.StatusNoContent {
		t.Fatalf("close: %d", rec.Code)
	}
	if s := sectionDetail(t, h, base, "sec-1")["status"]; s != "ready" {
		t.Fatalf("after closing: %v", s)
	}
	got := undoRoute(t, h, base, "sec-1")
	detail := sectionDetail(t, h, base, "sec-1")
	if got["status"] != "stale" || detail["status"] != "stale" || detail["text"] != "Texto de Síntesis." || chgOf(t, db, projectID, "sec-1") != change {
		t.Fatalf("undo: %v, detail %v", got, detail)
	}
}

// Rule 1: the paragraph that goes shares its first words with the next.
func TestB80_DeletingAParagraphWhoseNextStartsTheSameLeavesAnAlert(t *testing.T) {
	keep := "Primera parte larga que no cambia y sigue siendo larga para no pasar de la mitad."
	third := "La tercera parte queda igual y sigue siendo larga para no pasar de la mitad del texto aquí."
	second := "El plazo de entrega cae a dos semanas según el proveedor principal de la zona."
	closing := "La fecha de cierre es marzo y sigue así durante todo el contrato vigente."
	for name, c := range map[string]struct{ base, next string }{
		"first":                 {"El costo sube cada mes por el contrato nuevo.\n\n" + second + "\n\n" + third, second + "\n\n" + third},
		"middle":                {keep + "\n\nLa fecha es enero.\n\n" + closing, keep + "\n\n" + closing},
		"with a typo elsewhere": {keep + "\n\nLa fecha es enero.\n\n" + closing, keep + "\n\n" + strings.Replace(closing, "marzo", "abril", 1)},
	} {
		t.Run(name, func(t *testing.T) {
			h, db, _ := newAPI(t)
			projectID := newProject(t, h, nil)
			seedSignSection(t, db, projectID, c.base, false)
			base := "/api/projects/" + projectID
			saveOn(t, h, base, "sec-1", c.next)
			alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
			if len(alerts) != 1 || !hasReason(alerts[0], "1:paragraph_deleted") {
				t.Fatalf("alerts: %v", alerts)
			}
		})
	}
}

// Rule 2 reads the same list: the block's decision is named.
func TestB80_ABlockThatStartsLikeTheNextStillNamesItsDecision(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Cobramos por asiento. Se factura al mes.", "claim_origin": "material", "operation": "replace", "text": "Se negocia por cliente.\n\nSe factura al mes, y mucho, durante todo el contrato vigente con cada una de las empresas."}],
	  "new_decisions": [{"id": "new-1", "statement": "Negociar el descuento.", "instead_of": "Tarifa fija.", "would_choose_if": "Hubiera pocos clientes.", "assumption": false, "used_in_sections": ["sec-1"]}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("step: %+v", applied)
	}
	saveOn(t, h, base, "sec-1", "Se factura al mes, y mucho, durante todo el contrato vigente con cada una de las empresas.")
	alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || !hasReason(alerts[0], "1:paragraph_deleted") || !hasReason(alerts[0], "2:decision_text") {
		t.Fatalf("alerts: %v", alerts)
	}
}

// The counterparts: what keeps a paragraph's words deletes no paragraph.
func TestB80_MergingSplittingOrRewordingAParagraphLeavesNoAlert(t *testing.T) {
	one := "El costo sube cada mes por el contrato nuevo que firmamos con el proveedor principal."
	two := "El plazo de entrega cae a dos semanas según el mismo proveedor principal de la zona."
	for name, next := range map[string]string{
		"merged":               one + " " + two,
		"merged and corrected": one + " " + strings.Replace(two, "cae", "baja", 1),
		"split":                strings.Replace(one, " por el contrato", ".\n\nEs por el contrato", 1) + "\n\n" + two,
		"reworded":             strings.Replace(one, "sube", "aumenta", 1) + "\n\n" + two,
	} {
		t.Run(name, func(t *testing.T) {
			h, db, _ := newAPI(t)
			projectID := newProject(t, h, nil)
			seedSignSection(t, db, projectID, one+"\n\n"+two, false)
			base := "/api/projects/" + projectID
			saveOn(t, h, base, "sec-1", next)
			if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
				t.Fatalf("alerts: %v", alerts)
			}
		})
	}
}

const quotedSource = "El tribunal consideró que la tarifa, aplicada desde enero, no cumple el criterio de proporcionalidad exigido por la norma vigente en la materia."

// «una corrección de una coma… no»: a section that quotes a source minus a
// comma; adding it back adds no new fragment (rule 4).
func TestB80_RestoringACommaOfAQuotedSourceLeavesNoAlert(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	if rec := uploadReplacing(t, h, base, "", "fallo.txt", []byte(quotedSource)); rec.Code >= 300 {
		t.Fatalf("upload: %d", rec.Code)
	}
	seedSignSection(t, db, projectID, strings.Replace(quotedSource, "tarifa, aplicada", "tarifa aplicada", 1), false)
	saveOn(t, h, base, "sec-1", quotedSource)
	if alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		t.Fatalf("alerts: %v", alerts)
	}
}

// The counterpart: the same source, truly pasted, still alerts, also when the
// paste starts with the word that follows it.
func TestB80_PastingAQuoteOfASourceStillLeavesAnAlert(t *testing.T) {
	for name, tail := range map[string]string{"after": "Mi texto antes de la cita, propio y distinto.", "same first word": "El tribunal dijo más."} {
		t.Run(name, func(t *testing.T) {
			h, db, _ := newAPI(t)
			projectID := newProject(t, h, nil)
			base := "/api/projects/" + projectID
			if rec := uploadReplacing(t, h, base, "", "fallo.txt", []byte(quotedSource)); rec.Code >= 300 {
				t.Fatalf("upload: %d", rec.Code)
			}
			seedSignSection(t, db, projectID, tail, false)
			saveOn(t, h, base, "sec-1", quotedSource+" "+tail)
			alerts := projectEvents(t, db, base, store.EventSuspiciousEditAlert)
			if len(alerts) != 1 || !hasReason(alerts[0], "4:pasted") {
				t.Fatalf("alerts: %v", alerts)
			}
		})
	}
}

// The counterpart with a critical decision at its signed version: nothing
// differs from the signature, so the undo signs it back.
func TestB78_S4_WithTheSameCriticalDecisionTheUndoLeavesItSigned(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID
	v := openSignoffRoute(t, h, base)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "Solo comparo estudios con el mismo tamaño, porque si no la diferencia sale del filtro."}); rec.Code != http.StatusAccepted {
		t.Fatalf("explanation: %d", rec.Code)
	}
	waitRounds(t, h, base, v.Label, 1)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/answers", map[string]any{"origin_answer": "Del supuesto de que todos cuentan desde el mismo tamaño.", "case_answer": ""}); rec.Code != http.StatusAccepted {
		t.Fatalf("answers: %d", rec.Code)
	}
	waitRounds(t, h, base, v.Label, 2)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil); rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	saveOn(t, h, base, "sec-1", plainText+" Un añadido mío.")
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "signed" {
		t.Fatalf("undo: %v", got)
	}
}
