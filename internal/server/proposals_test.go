package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 4.1: proposals (B-73 to B-77), through the routes.

const proposalText = "Cobramos por asiento. Se factura al mes."

func proposalsAPI(t *testing.T) (http.Handler, *store.DB, string, string) {
	t.Helper()
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedSignSection(t, db, projectID, proposalText, false)
	return h, db, "/api/projects/" + projectID, projectID
}

func stepOn(t *testing.T, db *store.DB, projectID, step string) cycle.ApplyResult {
	t.Helper()
	gate := cycle.NewGate(db)
	res, err := gate.EndStep(context.Background(), projectID, "turn-1", 1, json.RawMessage(step))
	if err != nil || res.Status != cycle.StatusAccepted {
		t.Fatalf("EndStep: %+v, %v", res, err)
	}
	applied, err := gate.ApplyStep(context.Background(), projectID, "turn-1", json.RawMessage(step))
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	return applied
}

func replaceStep(base int, anchor, text string) string {
	return `{"stretch": "build", "triage_line": "Reescribo el cobro.", "closing_risk": "none", "note": "Nota del paso.",
	  "new_text": [{"section_id": "sec-1", "base_version": ` + itoa(base) + `, "anchor": "` + anchor + `", "claim_origin": "material",
	    "operation": "replace", "text": "` + text + `"}]}`
}

func listProposals(t *testing.T, h http.Handler, base, query string) []map[string]any {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/proposals"+query, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET proposals: %d %s", rec.Code, rec.Body.String())
	}
	return decode[[]map[string]any](t, rec)
}

func sectionDetail(t *testing.T, h http.Handler, base, label string) map[string]any {
	t.Helper()
	rec := do(t, h, http.MethodGet, base+"/sections/"+label, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET section: %d %s", rec.Code, rec.Body.String())
	}
	return decode[map[string]any](t, rec)
}

func saveText(t *testing.T, h http.Handler, base string, version int, text string) {
	t.Helper()
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": version, "text": text})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
}

func act(t *testing.T, h http.Handler, base, label, verb string) map[string]any {
	t.Helper()
	rec := do(t, h, http.MethodPost, base+"/proposals/"+label+"/"+verb, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", verb, label, rec.Code, rec.Body.String())
	}
	return decode[map[string]any](t, rec)
}

func signSection(t *testing.T, db *store.DB, projectID string) {
	t.Helper()
	gate := cycle.NewGate(db)
	open, err := gate.RequestSignoff(context.Background(), projectID, "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if err := gate.RecordExplanation(context.Background(), projectID, open.Label, "Lo explico yo."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if !open.Brief {
		// A decision that is not critical is the thesis (B-106): one Wonder
		// round about it, with no origin question.
		out := `{"decisions": [{"decision": "dec-1", "verdict": "match", "missing": null, "passage": null}],
			"origin_question": {"question": "not_applicable", "answer": null}, "previous_case": null, "new_case": null}`
		res, err := gate.EndSignoffRound(context.Background(), projectID, "turn-w", open.Label, 1, json.RawMessage(out), "wonder high", false)
		if err != nil || res.Outcome != cycle.OutcomeCoincides {
			t.Fatalf("EndSignoffRound: %+v, %v", res, err)
		}
	}
	if err := gate.Sign(context.Background(), projectID, open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// «tú editas «Modelo de cobro» y el paso del agente sobre la versión
// anterior queda como propuesta; tu edición rige (caso 9)».
func TestB73a_AStepOnWhatYouEditedIsAProposalAndYourEditRules(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	mine := "Cobramos por licencia. Se factura al mes."
	saveText(t, h, base, 1, mine)

	applied := stepOn(t, db, projectID, replaceStep(1, "Cobramos por asiento.", "Cobramos por uso."))
	if !applied.Applied || len(applied.Rejected) != 0 || len(applied.Errors) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != mine || detail["version"] != float64(2) {
		t.Fatalf("your edit must rule: %v", detail)
	}
	list := listProposals(t, h, base, "?section=sec-1")
	if len(list) != 1 {
		t.Fatalf("proposals: %v", list)
	}
	p := list[0]
	if p["label"] != "prop-1" || p["state"] != "open" || p["section"] != "sec-1" || p["base_version"] != float64(1) ||
		p["anchor"] != "Cobramos por asiento." || p["operation"] != "replace" || p["text"] != "Cobramos por uso." {
		t.Fatalf("proposal: %v", p)
	}
	contrast, _ := p["contrast"].(map[string]any)
	if contrast["triage_line"] != "Reescribo el cobro." || contrast["note"] != "Nota del paso." {
		t.Fatalf("contrast: %v", contrast)
	}
	if removed, _ := contrast["removed"].([]any); len(removed) != 1 || removed[0] != "Cobramos por asiento." {
		t.Fatalf("contrast removed: %v", contrast)
	}
	if added, _ := contrast["added"].([]any); len(added) != 1 || added[0] != "Cobramos por uso." {
		t.Fatalf("contrast added: %v", contrast)
	}
	// One line in «Lo que te toca», which the count includes.
	frame := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/frame", nil))
	todo, _ := frame["todo"].(map[string]any)
	if lines, _ := todo["proposals"].([]any); len(lines) != 1 {
		t.Fatalf("todo: %v", todo)
	}
}

// B-74: the contrast carries the step's new decisions for that section,
// and the view says when the current version changed since it was made.
func TestB74_TheContrastIsStoredWithItsDecisionsAndTheViewSaysTheVersionMoved(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 2, "anchor": "Se factura al mes.", "claim_origin": "material", "operation": "replace", "text": "Se factura al año."}],
	  "new_decisions": [{"id": "new-1", "statement": "Facturar al año.", "instead_of": "Facturar al mes.", "would_choose_if": "Hubiera caja.", "assumption": false, "used_in_sections": ["sec-1"]}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	p := listProposals(t, h, base, "?section=sec-1")[0]
	contrast, _ := p["contrast"].(map[string]any)
	decisions, _ := contrast["decisions"].([]any)
	if len(decisions) != 1 || decisions[0].(map[string]any)["instead_of"] != "Facturar al mes." {
		t.Fatalf("contrast decisions: %v", contrast)
	}
	if p["version_changed"] != false {
		t.Fatalf("version_changed at birth: %v", p)
	}
	saveText(t, h, base, int(sectionDetail(t, h, base, "sec-1")["version"].(float64)), "Otro texto mío.")
	if p := listProposals(t, h, base, "?section=sec-1")[0]; p["version_changed"] != true {
		t.Fatalf("version_changed after your edit: %v", p)
	}
}

// «un paso sobre una sección firmada deja una propuesta y la sección
// sigue signed; aceptarla la pasa a changed_since_signoff».
func TestB73b_AStepOnASignedSectionLeavesAProposalAndAcceptingMovesIt(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))

	applied := stepOn(t, db, projectID, replaceStep(version, "Se factura al mes.", "Se factura al año."))
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "signed" || detail["text"] != proposalText {
		t.Fatalf("the section must stay signed: %v", detail)
	}
	got := act(t, h, base, "prop-1", "accept")
	if got["state"] != "accepted" {
		t.Fatalf("accept: %v", got)
	}
	detail = sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "changed_since_signoff" || detail["text"] != "Cobramos por asiento. Se factura al año." {
		t.Fatalf("after accepting: %v", detail)
	}
}

// P-40's rule, replaced: in «cambió», steps arrive as proposals too.
func TestB73b_AStepOnAChangedSinceSignoffSectionLeavesAProposal(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	saveText(t, h, base, version, proposalText+" Y más.")

	applied := stepOn(t, db, projectID, replaceStep(version+1, "Se factura al mes.", "Se factura al año."))
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "changed_since_signoff" || detail["text"] != proposalText+" Y más." {
		t.Fatalf("section: %v", detail)
	}
	if list := listProposals(t, h, base, "?section=sec-1"); len(list) != 1 || list[0]["state"] != "open" {
		t.Fatalf("proposals: %v", list)
	}
}

// «un paso sobre una sección en firma deja una propuesta y la firma sigue»
// (K-16), and it can still be signed: a proposal blocks nothing (estados §1).
func TestK16_AStepOnASectionInSignoffLeavesAProposalAndTheSignoffGoesOn(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	gate := cycle.NewGate(db)
	open, err := gate.RequestSignoff(context.Background(), projectID, "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	applied := stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año."))
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	if list := listProposals(t, h, base, "?section=sec-1"); len(list) != 1 {
		t.Fatalf("proposals: %v", list)
	}
	view := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/signoffs/"+open.Label, nil))
	if view["phase"] != "in_progress" {
		t.Fatalf("the signoff must go on: %v", view)
	}
	if err := gate.RecordExplanation(context.Background(), projectID, open.Label, "Lo explico yo."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if err := gate.Sign(context.Background(), projectID, open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
}

// «aceptar con el ancla ya repetida no aplica nada y pide un paso nuevo»
// (B-75, T-4): the proposal goes to discarded with that reason, and the
// principal's next step is told so.
func TestB75_AcceptingWithARepeatedAnchorAppliesNothingAndAsksForANewStep(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	if applied := stepOn(t, db, projectID, replaceStep(version, "Se factura al mes.", "Se factura al año.")); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	repeated := "Se factura al mes. Otra vez: Se factura al mes."
	saveText(t, h, base, version, repeated)
	after := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))

	got := act(t, h, base, "prop-1", "accept")
	if got["state"] != "discarded" || got["why"] != "anchor_not_unique" {
		t.Fatalf("accept: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != repeated || detail["version"] != float64(after) {
		t.Fatalf("nothing must apply: %v", detail)
	}
	list := listProposals(t, h, base, "?section=sec-1")
	if list[0]["state"] != "discarded" || list[0]["why"] != "anchor_not_unique" {
		t.Fatalf("proposal: %v", list[0])
	}
}

// «aceptar una propuesta que declara chg-1 atendido lo cierra» (A-1), and
// «un «actualizado» de chg-1 en un paso que fue a propuesta no se rechaza y
// se aplica al aceptarla».
func TestA1_AnUpdatedCloseOnAProposalIsNotRejectedAndAcceptingClosesIt(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), projectID, "sec-1")
		if err != nil {
			return err
		}
		label, err := tx.NewChangeLabel()
		if err != nil {
			return err
		}
		if label != "chg-1" {
			t.Errorf("label: %s", label)
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["open_changes"] = []any{map[string]any{"label": label, "about": "dec-9"}}
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "changed_since_signoff", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "app"})
		return err
	})
	if err != nil {
		t.Fatalf("seed change: %v", err)
	}
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	step := strings.TrimSuffix(replaceStep(version, "Se factura al mes.", "Se factura al año."), "}") +
		`, "handled_changes": [{"change_id": "chg-1", "result": "updated", "reason": ""}]}`
	applied := stepOn(t, db, projectID, step)
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("the close must not be rejected: %+v", applied)
	}
	if changes := openChangesOf(t, db, projectID); len(changes) != 1 {
		t.Fatalf("still open until it is accepted: %v", changes)
	}
	if got := act(t, h, base, "prop-1", "accept"); got["state"] != "accepted" {
		t.Fatalf("accept: %v", got)
	}
	if changes := openChangesOf(t, db, projectID); len(changes) != 0 {
		t.Fatalf("accepting must close it: %v", changes)
	}
	closes := projectEvents(t, db, base, store.EventUsageChange)
	if last := closes[len(closes)-1]; last["kind"] != "updated" || last["change"] != "chg-1" {
		t.Fatalf("close event: %v", last)
	}
}

func openChangesOf(t *testing.T, db *store.DB, projectID string) []any {
	t.Helper()
	sec, err := db.GetObject(context.Background(), projectID, "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	_ = json.Unmarshal(sec.Body, &b)
	out, _ := b["open_changes"].([]any)
	return out
}

// «firmar con una propuesta abierta la deja discarded (signed_without)»
// (B-76), listed before in the signoff, and it may sit in history.
func TestB76_SigningWithAnOpenProposalLeavesItDiscardedSignedWithout(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	if applied := stepOn(t, db, projectID, replaceStep(1, "Cobramos por asiento.", "Cobramos por uso.")); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	gate := cycle.NewGate(db)
	open, err := gate.RequestSignoff(context.Background(), projectID, "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("a proposal must not block asking to sign: %+v, %v", open, err)
	}
	view := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/signoffs/"+open.Label, nil))
	if listed, _ := view["open_proposals"].([]any); len(listed) != 1 {
		t.Fatalf("the signoff must list it: %v", view)
	}
	if err := gate.RecordExplanation(context.Background(), projectID, open.Label, "Lo explico yo."); err != nil {
		t.Fatalf("RecordExplanation: %v", err)
	}
	if err := gate.Sign(context.Background(), projectID, open.Label); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	p := listProposals(t, h, base, "?section=sec-1")[0]
	if p["state"] != "discarded" || p["why"] != "signed_without" {
		t.Fatalf("proposal: %v", p)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["status"] != "signed" ||
		detail["text"] != "Cobramos por licencia. Se factura al mes." {
		t.Fatalf("section: %v", detail)
	}
	var found bool
	for _, data := range projectEvents(t, db, base, store.EventStateChanged) {
		if data["to"] == "discarded" && data["why"] == "signed_without" && data["label"] == "prop-1" && data["section"] == "sec-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("no event in the history for the discard at signing")
	}
}

const newSectionStep = `{"stretch": "build", "triage_line": "Propongo.", "closing_risk": "none", "note": "N.",
  "new_text": [{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Texto de los riesgos."}],
  "proposed_roles": [{"section_id": "new-1", "role": "conclusion", "title": "Riesgos"}],
  "new_decisions": [{"id": "new-2", "statement": "Listar riesgos.", "instead_of": "Omitirlos.", "would_choose_if": "No hubiera ninguno.", "assumption": false, "used_in_sections": ["new-1"]}]}`

// «con el plan confirmado, una sección new-n del agente queda como
// propuesta de sección» (B-77): accepting creates it as a step does before
// the plan; discarding creates nothing.
func TestB77_WithThePlanConfirmedANewSectionIsASectionProposal(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if _, err := cycle.ConfirmPlan(context.Background(), db, projectID); err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	applied := stepOn(t, db, projectID, newSectionStep)
	if !applied.Applied || len(applied.Rejected) != 0 || applied.Labels["new-1"] != "" {
		t.Fatalf("applied: %+v", applied)
	}
	if sections := decode[[]map[string]any](t, do(t, h, http.MethodGet, base+"/sections", nil)); len(sections) != 1 {
		t.Fatalf("no section yet: %v", sections)
	}
	list := listProposals(t, h, base, "")
	ns, _ := list[0]["new_section"].(map[string]any)
	if len(list) != 1 || ns["title"] != "Riesgos" || ns["role"] != "conclusion" || list[0]["text"] != "Texto de los riesgos." {
		t.Fatalf("proposal: %v", list)
	}
	frame := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/frame", nil))
	todo, _ := frame["todo"].(map[string]any)
	if lines, _ := todo["proposals"].([]any); len(lines) != 1 {
		t.Fatalf("todo: %v", todo)
	}
	got := act(t, h, base, "prop-1", "accept")
	if got["state"] != "accepted" || got["section"] != "sec-2" {
		t.Fatalf("accept: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-2")
	if detail["title"] != "Riesgos" || detail["text"] != "Texto de los riesgos." || detail["role"] != "conclusion" ||
		detail["role_state"] != "proposed" {
		t.Fatalf("new section: %v", detail)
	}
	// The step's decision used in it is linked now.
	dec := decisionUsedIn(t, db, projectID, "dec-1", "sec-2")
	if !dec {
		t.Fatal("the decision declared for the new section must be linked at accepting")
	}
}

func decisionUsedIn(t *testing.T, db *store.DB, projectID, decision, section string) bool {
	t.Helper()
	dec, err := db.GetObject(context.Background(), projectID, decision)
	if err != nil {
		t.Fatal(err)
	}
	sec, err := db.GetObject(context.Background(), projectID, section)
	if err != nil {
		t.Fatal(err)
	}
	edges, err := db.EdgesFrom(context.Background(), projectID, dec.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Type == store.EdgeUsedIn && e.ToID == sec.ID {
			return true
		}
	}
	return false
}

func TestB77_DiscardingASectionProposalCreatesNothing(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if _, err := cycle.ConfirmPlan(context.Background(), db, projectID); err != nil {
		t.Fatalf("ConfirmPlan: %v", err)
	}
	stepOn(t, db, projectID, newSectionStep)
	got := act(t, h, base, "prop-1", "discard")
	if got["state"] != "discarded" {
		t.Fatalf("discard: %v", got)
	}
	if sections := decode[[]map[string]any](t, do(t, h, http.MethodGet, base+"/sections", nil)); len(sections) != 1 {
		t.Fatalf("discarding must create nothing: %v", sections)
	}
	if rec := do(t, h, http.MethodPost, base+"/proposals/prop-1/accept", nil); rec.Code != http.StatusConflict {
		t.Fatalf("accepting a closed proposal: %d", rec.Code)
	}
}

// «el resto del paso (decisiones, vínculos) se aplica igual».
func TestB73_TheRestOfTheStepApplies(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if rec := do(t, h, http.MethodPost, base+"/sections", map[string]any{"title": "Otra", "role": "", "focus": ""}); rec.Code >= 300 {
		t.Fatalf("second section: %d %s", rec.Code, rec.Body.String())
	}
	signSection(t, db, projectID)
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [
	    {"section_id": "sec-1", "base_version": ` + itoa(version) + `, "anchor": "Se factura al mes.", "claim_origin": "material", "operation": "replace", "text": "Se factura 2 veces al año."},
	    {"section_id": "sec-2", "base_version": 1, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Texto de la otra."}],
	  "new_decisions": [{"id": "new-1", "statement": "Facturar dos veces.", "instead_of": "Una vez.", "would_choose_if": "Hubiera caja.", "assumption": false, "used_in_sections": ["sec-1", "sec-2"]}],
	  "figure_links": [{"figure_anchor": "2 veces", "decision_id": "new-1"}]}`
	applied := stepOn(t, db, projectID, step)
	if !applied.Applied || len(applied.Rejected) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	if detail := sectionDetail(t, h, base, "sec-2"); detail["text"] != "Texto de la otra." {
		t.Fatalf("sec-2: %v", detail)
	}
	if !decisionUsedIn(t, db, projectID, "dec-1", "sec-1") || !decisionUsedIn(t, db, projectID, "dec-1", "sec-2") {
		t.Fatal("the decision and its links must apply")
	}
	// The figure anchored only in the proposed text goes with the proposal
	// and comes back at accepting (B-73).
	if figures, _ := db.ListObjects(context.Background(), projectID, store.ObjectFigure); len(figures) != 0 {
		t.Fatalf("figures before accepting: %d", len(figures))
	}
	p := listProposals(t, h, base, "?section=sec-1")[0]
	if figures, _ := p["figures"].([]any); len(figures) != 1 {
		t.Fatalf("proposal figures: %v", p)
	}
	act(t, h, base, "prop-1", "accept")
	if figures, _ := db.ListObjects(context.Background(), projectID, store.ObjectFigure); len(figures) != 1 {
		t.Fatalf("figures after accepting: %d", len(figures))
	}
	if detail := sectionDetail(t, h, base, "sec-1"); len(detail["figures"].([]any)) != 1 {
		t.Fatalf("section figures: %v", detail)
	}
}

// «el agente escribe sobre la v3 y tú guardas desde la v3: rige tu texto y lo
// del agente queda como propuesta» (espec. §2, cuarta fila), a single
// replace_all with its figures and citations, which leave and come back.
func TestB73d_YourSaveOverTheAgentsVersionRulesAndItsTextIsOneProposal(t *testing.T) {
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
	first := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Cobramos por asiento.", "claim_origin": "material", "operation": "replace", "text": "Cobramos 3 veces por uso."}],
	  "new_decisions": [{"id": "new-1", "statement": "Cobrar por uso.", "instead_of": "Por asiento.", "would_choose_if": "Hubiera asientos.", "assumption": false, "used_in_sections": ["sec-1"]}],
	  "figure_links": [{"figure_anchor": "3 veces", "decision_id": "new-1"}],
	  "citations": [{"claim_anchor": "por uso", "source_id": "src-1", "passage": "el uso manda"}]}`
	if applied := stepOn(t, db, projectID, first); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("first: %+v", applied)
	}
	if applied := stepOn(t, db, projectID, replaceStep(2, "Se factura al mes.", "Se factura al año.")); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("second: %+v", applied)
	}
	agents := sectionDetail(t, h, base, "sec-1")["text"].(string)
	if agents != "Cobramos 3 veces por uso. Se factura al año." {
		t.Fatalf("agent's text: %q", agents)
	}

	// You save from version 1, the one your tab read.
	mine := "Cobramos por licencia."
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": mine})
	if rec.Code != http.StatusOK {
		t.Fatalf("your save over the agent's version: %d %s", rec.Code, rec.Body.String())
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != mine {
		t.Fatalf("your text must rule: %v", detail)
	}
	if len(detail["figures"].([]any)) != 0 || len(detail["citations"].([]any)) != 0 {
		t.Fatalf("the agent's figures and citations leave the current text: %v", detail)
	}
	list := listProposals(t, h, base, "?section=sec-1")
	if len(list) != 1 {
		t.Fatalf("one proposal, however many steps: %v", list)
	}
	p := list[0]
	if p["operation"] != "replace_all" || p["anchor"] != "" || p["text"] != agents {
		t.Fatalf("proposal: %v", p)
	}
	if figures, _ := p["figures"].([]any); len(figures) != 1 {
		t.Fatalf("proposal figures: %v", p)
	}
	if citations, _ := p["citations"].([]any); len(citations) != 1 {
		t.Fatalf("proposal citations: %v", p)
	}
	// Your edit was over the agent's text: no R4 card for what you did not read.
	if edits := projectEvents(t, db, base, store.EventEdit); edits[len(edits)-1]["_actor"] != "human" {
		t.Fatalf("edits: %v", edits)
	}

	if got := act(t, h, base, "prop-1", "accept"); got["state"] != "accepted" {
		t.Fatalf("accept: %v", got)
	}
	detail = sectionDetail(t, h, base, "sec-1")
	if detail["text"] != agents || len(detail["figures"].([]any)) != 1 || len(detail["citations"].([]any)) != 1 {
		t.Fatalf("after accepting: %v", detail)
	}
	if linked := detail["citations"].([]any)[0].(map[string]any); linked["target"] != "src-1" {
		t.Fatalf("the citation comes back linked: %v", linked)
	}
}

// B-73 d: if the section changed after your save, accepting asks for a new
// step, like B-75.
func TestB73d_AcceptingAfterTheSectionChangedAsksForANewStep(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if applied := stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año.")); !applied.Applied {
		t.Fatalf("applied: %+v", applied)
	}
	saveText(t, h, base, 1, "Mío.")
	saved := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	saveText(t, h, base, saved, "Mío otra vez.")

	got := act(t, h, base, "prop-1", "accept")
	if got["state"] != "discarded" || got["why"] != "section_changed" {
		t.Fatalf("accept: %v", got)
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Mío otra vez." {
		t.Fatalf("nothing must apply: %v", detail)
	}
}

// B-73 d: a save that changes nothing from what your tab read is no edit:
// the agent's text stays.
func TestB73d_ASaveOfTheVersionYouReadOverTheAgentsChangesNothing(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año."))
	rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": proposalText})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if detail := sectionDetail(t, h, base, "sec-1"); detail["text"] != "Cobramos por asiento. Se factura al año." {
		t.Fatalf("section: %v", detail)
	}
	if list := listProposals(t, h, base, ""); len(list) != 0 {
		t.Fatalf("proposals: %v", list)
	}
}

// «un paso del agente sobre una versión que cambió otro paso suyo sigue con
// el rechazo y el reintento de B-11, sin propuesta».
func TestB11_AStepOnAVersionItsOwnStepChangedIsStillRejectedWithoutAProposal(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	if applied := stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año.")); !applied.Applied {
		t.Fatalf("first: %+v", applied)
	}
	applied := stepOn(t, db, projectID, replaceStep(1, "Cobramos por asiento.", "Cobramos por uso."))
	if applied.Applied || len(applied.Errors) != 1 || !strings.Contains(applied.Errors[0], "changed since the version you read") {
		t.Fatalf("applied: %+v", applied)
	}
	if list := listProposals(t, h, base, ""); len(list) != 0 {
		t.Fatalf("proposals: %v", list)
	}
}

// «aceptar registra edit con actor: agent y accepted_by: human, y «Deshacer»
// queda apagado» (B-75, B-79).
func TestB75_AcceptingRecordsAnEditOfTheAgentAcceptedByYouAndUndoIsOff(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	saveText(t, h, base, 1, "Cobramos por licencia. Se factura al mes.")
	if detail := sectionDetail(t, h, base, "sec-1"); detail["undo_available"] != true {
		t.Fatalf("after your edit, undo is on: %v", detail)
	}
	stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año."))
	if got := act(t, h, base, "prop-1", "accept"); got["state"] != "accepted" {
		t.Fatalf("accept: %v", got)
	}
	edits := projectEvents(t, db, base, store.EventEdit)
	last := edits[len(edits)-1]
	if last["_actor"] != "agent" || last["accepted_by"] != "human" {
		t.Fatalf("edit event: %v", last)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["text"] != "Cobramos por licencia. Se factura al año." || detail["undo_available"] != false {
		t.Fatalf("after accepting: %v", detail)
	}
}

// Discarding asks for no «porque» (B-75) and only the proposal moves.
func TestB75_DiscardingAsksNoReasonAndLeavesTheSectionAlone(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	signSection(t, db, projectID)
	version := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	stepOn(t, db, projectID, replaceStep(version, "Se factura al mes.", "Se factura al año."))
	if got := act(t, h, base, "prop-1", "discard"); got["state"] != "discarded" {
		t.Fatalf("discard: %v", got)
	}
	detail := sectionDetail(t, h, base, "sec-1")
	if detail["status"] != "signed" || detail["version"] != float64(version) {
		t.Fatalf("section: %v", detail)
	}
	if rec := do(t, h, http.MethodPost, base+"/proposals/nope-1/discard", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown proposal: %d", rec.Code)
	}
}

// Accepting on a section in signoff changes its text, so it cancels the
// signoff like your edit (K-16, the choice in elecciones-agente.md).
func TestK16_AcceptingOnASectionInSignoffCancelsTheSignoff(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), projectID, "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año."))
	act(t, h, base, "prop-1", "accept")
	view := decode[map[string]any](t, do(t, h, http.MethodGet, base+"/signoffs/"+open.Label, nil))
	if view["phase"] != "cancelled" {
		t.Fatalf("the signoff: %v", view)
	}
}

// B-75: through the route, with no turn running, accepting what cannot apply
// opens a turn for the new step.
func TestB75_TheRouteOpensATurnForTheNewStep(t *testing.T) {
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "app", "04-principal-no-work.json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	hub := server.NewHub()
	runner := cycle.NewRunner(db, provider.NewFake(fx), nil, hub)
	t.Cleanup(runner.Close)
	h := withVisitor(t, db, server.NewWithAPI(nil, hub, db, runner))
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, proposalText, false)
	stepOn(t, db, projectID, replaceStep(1, "Se factura al mes.", "Se factura al año."))
	saveText(t, h, base, 1, "Mío.")
	saved := int(sectionDetail(t, h, base, "sec-1")["version"].(float64))
	saveText(t, h, base, saved, "Mío otra vez.")
	got := act(t, h, base, "prop-1", "accept")
	if got["why"] != "section_changed" || got["turn_id"] == nil || got["turn_id"] == "" {
		t.Fatalf("accept: %v", got)
	}
}

// B-74: the proposal your save displaced carries, in its contrast, the
// decisions of the steps it displaced that the section uses.
func TestB74_TheDisplacedContrastCarriesTheDecisionsOfThoseSteps(t *testing.T) {
	h, db, base, projectID := proposalsAPI(t)
	step := `{"stretch": "build", "triage_line": "T.", "closing_risk": "none", "note": "N.",
	  "new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "Cobramos por asiento.", "claim_origin": "material", "operation": "replace", "text": "Cobramos por uso."}],
	  "new_decisions": [{"id": "new-1", "statement": "Cobrar por uso.", "instead_of": "Por asiento.", "would_choose_if": "Hubiera asientos.", "assumption": false, "used_in_sections": ["sec-1"]}]}`
	if applied := stepOn(t, db, projectID, step); !applied.Applied || len(applied.Errors) != 0 {
		t.Fatalf("applied: %+v", applied)
	}
	saveText(t, h, base, 1, "Cobramos por licencia.")
	p := listProposals(t, h, base, "?section=sec-1")[0]
	contrast, _ := p["contrast"].(map[string]any)
	decisions, _ := contrast["decisions"].([]any)
	if p["origin"] != "displaced" || len(decisions) != 1 {
		t.Fatalf("contrast: %v", p)
	}
	d := decisions[0].(map[string]any)
	if d["label"] != "dec-1" || d["statement"] != "Cobrar por uso." || d["instead_of"] != "Por asiento." {
		t.Fatalf("decision: %v", d)
	}
}
