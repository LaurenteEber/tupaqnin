package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.1 (backlog-r2): the app's rules after each step and each edit.

func lastStepEnded(t *testing.T, db *store.DB) map[string]any {
	t.Helper()
	all := stepEndedData(t, db)
	if len(all) == 0 {
		t.Fatal("no step_ended")
	}
	return all[len(all)-1]
}

func hasPending(pendings []graph.Pending, kind, about string) bool {
	for _, p := range pendings {
		if p.Kind == kind && (about == "" || p.About == about) {
			return true
		}
	}
	return false
}

// B-43, rule 0: a direct step that touches a figure is reclassified as
// build in its step_ended, which the triage line shows, and goes through
// every rule: the loose number is pending (rule 1) and the critical
// decision gets its card (rule 3).
func TestB43_DirectStepTouchingAFigureBecomesBuild(t *testing.T) {
	db := seedStepProject(t)
	out := strings.Replace(string(stepFile(t, "figure-datum-loose")), `"stretch": "build"`, `"stretch": "direct"`, 1)
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied || applied.Stretch != "build" ||
		strings.Join(applied.Reclassified, ",") != "figure" {
		t.Fatalf("ApplyStep: %+v", applied)
	}
	data := lastStepEnded(t, db)
	if data["stretch"] != "build" || data["proposed_stretch"] != "direct" {
		t.Fatalf("step_ended: stretch %v, proposed %v; want build from direct", data["stretch"], data["proposed_stretch"])
	}
	if reasons, _ := data["reclassified"].([]any); len(reasons) != 1 || reasons[0] != checks.TouchesFigure {
		t.Fatalf("step_ended reclassified: %v", data["reclassified"])
	}
	_, pendings := status(t, db, "sec-1")
	if !hasPending(pendings, graph.PendingFigureWithoutDecision, "7") || !hasPending(pendings, graph.PendingOpenCard, "card-1") {
		t.Fatalf("pendings: %v, want the loose 7 and card-1", pendings)
	}
}

// B-43: a direct step that touches nothing on the list stays direct.
func TestB43_DirectStepTouchingNothingStaysDirect(t *testing.T) {
	db := seedStepProject(t)
	out := `{"stretch":"direct","triage_line":"Vía directa: corrijo una frase.","closing_risk":"none","note":"n",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Fin."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied || applied.Stretch != "direct" || applied.Reclassified != nil {
		t.Fatalf("ApplyStep: %+v", applied)
	}
	if data := lastStepEnded(t, db); data["stretch"] != "direct" || data["proposed_stretch"] != nil {
		t.Fatalf("step_ended: %v", data)
	}
}

// T-2, rule 5: dec-2 in the text of a step, or new-2 in your edit, is
// «códigos internos»; «Formulario F-40» is not. It closes when the code
// leaves the text.
func TestT2_InternalCodeInAStepOrYourEditIsAPending(t *testing.T) {
	db := seedStepProject(t)
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Según dec-2 y el Formulario F-40."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied {
		t.Fatalf("ApplyStep: %v", applied.Errors)
	}
	_, pendings := status(t, db, "sec-1")
	if !hasPending(pendings, graph.PendingInternalCodes, "dec-2") || hasPending(pendings, graph.PendingInternalCodes, "f-40") {
		t.Fatalf("pendings after the step: %v, want dec-2 and not F-40", pendings)
	}

	findings := yourEdit(t, db, "sec-1", "La síntesis compara estudios. Los estudios usan filtros. Lo dijo new-2 en el Formulario F-40.")
	if !hasFinding(findings, 5, graph.PendingInternalCodes, "new-2") || hasFinding(findings, 5, graph.PendingInternalCodes, "f-40") {
		t.Fatalf("findings of your edit: %+v", findings)
	}
	_, pendings = status(t, db, "sec-1")
	if !hasPending(pendings, graph.PendingInternalCodes, "new-2") || hasPending(pendings, graph.PendingInternalCodes, "dec-2") {
		t.Fatalf("pendings after your edit: %v, want new-2 only", pendings)
	}
	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Lo dijo el Formulario F-40.")
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingInternalCodes, "") {
		t.Fatalf("pendings after removing the code: %v", pendings)
	}
}

// B-42, rule 2: a step in «Conclusiones» with no declared decision
// leaves «escrito sin decisión»; one with a decision used_in that
// section does not, and closes it.
func TestB42_ConclusionStepWithoutDecisionLeavesTextWithoutDecision(t *testing.T) {
	db := seedStepProject(t)
	first := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"proposed_roles":[{"section_id":"new-1","role":"conclusion","title":"Conclusiones"}],
		"new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"material","operation":"insert_after","text":"La embotellada trae más partículas."}]}`
	applied := applyRecorded(t, db, json.RawMessage(first))
	if !applied.Applied || applied.Labels["new-1"] != "sec-2" {
		t.Fatalf("first step: %+v", applied)
	}
	if data := lastStepEnded(t, db); strings.Join(anyStrings(data["text_without_decision"]), ",") != "sec-2" {
		t.Fatalf("step_ended text_without_decision: %v", data["text_without_decision"])
	}
	if _, pendings := status(t, db, "sec-2"); !hasPending(pendings, graph.PendingTextWithoutDecision, "") {
		t.Fatalf("sec-2 pendings: %v, want text_without_decision", pendings)
	}

	second := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_decisions":[{"id":"new-1","statement":"Concluir por veces.","instead_of":"Concluir por conteos.",
			"would_choose_if":"Todos usaran el mismo filtro.","assumption":false,"used_in_sections":["sec-2"]}],
		"new_text":[{"section_id":"sec-2","base_version":1,"anchor":"más partículas.","claim_origin":"material","operation":"replace","text":"más partículas, por veces."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(second)); !applied.Applied {
		t.Fatalf("second step: %v", applied.Errors)
	}
	data := lastStepEnded(t, db)
	if len(anyStrings(data["text_without_decision"])) != 0 || strings.Join(anyStrings(data["decided_sections"]), ",") != "sec-2" {
		t.Fatalf("second step_ended: %v", data)
	}
	if _, pendings := status(t, db, "sec-2"); hasPending(pendings, graph.PendingTextWithoutDecision, "") {
		t.Fatalf("sec-2 pendings: %v, want it closed", pendings)
	}
}

// B-42: rule 2 only sees conclusion sections; sec-1 has role other.
func TestB42_OtherSectionWithoutDecisionIsLeftToTheChecker(t *testing.T) {
	db := seedStepProject(t)
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"Fin."}]}`
	applyRecorded(t, db, json.RawMessage(out))
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingTextWithoutDecision, "") {
		t.Fatalf("sec-1 pendings: %v", pendings)
	}
}

// yourEdit saves your text on a section, in one transaction with the
// app's rules, as the editor's save does (B-36).
func yourEdit(t *testing.T, db *store.DB, label, text string) []checks.Finding {
	t.Helper()
	ctx := context.Background()
	var findings []checks.Finding
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", label)
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		base, _ := b["text"].(string)
		b["text"] = text
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b,
			store.EventInput{Type: store.EventEdit, Actor: "human"}); err != nil {
			return err
		}
		findings, err = cycle.CheckYourEdit(ctx, tx, sec.ID, base)
		return err
	})
	if err != nil {
		t.Fatalf("your edit: %v", err)
	}
	return findings
}

func hasFinding(findings []checks.Finding, rule int, kind, about string) bool {
	for _, f := range findings {
		if f.Rule == rule && f.Kind == kind && (about == "" || f.About == about) {
			return true
		}
	}
	return false
}

func anyStrings(v any) []string {
	items, _ := v.([]any)
	out := []string{}
	for _, item := range items {
		s, _ := item.(string)
		out = append(out, s)
	}
	return out
}

// B-49, R4: your edit that changes a figure covered by a critical
// decision opens its card, whatever the decision's state; the card asks
// only for the «porque» (option null), the decision keeps its state and
// the section stays pending until you answer.
func TestB49_YourEditOnACriticalFigureOpensTheR4CardClosedOnlyWithReason(t *testing.T) {
	db := cleanCardProject(t)
	gate := cycle.NewGate(db)
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara igual."); err != nil {
		t.Fatalf("answer card-1: %v", err)
	}
	_, sb := object(t, db, "sec-1")
	edited := strings.Replace(sb["text"].(string), "60 veces", "61 veces", 1)
	findings := yourEdit(t, db, "sec-1", edited)
	if !hasFinding(findings, 3, checks.FindingCriticalEdit, "60 veces") {
		t.Fatalf("findings: %+v", findings)
	}
	card, cb := object(t, db, "card-2")
	if card.State != "open" || cb["kind"] != cycle.CardOwnEdit || cb["decision_id"] == "" {
		t.Fatalf("card-2: %s %v", card.State, cb)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingOpenCard, "card-2") {
		t.Fatalf("sec-1 pendings: %v, want card-2", pendings)
	}
	// Editing again while it is open does not open another one.
	yourEdit(t, db, "sec-1", strings.Replace(edited, "61 veces", "62 veces", 1))
	if cards, _ := db.ListObjects(context.Background(), "p1", store.ObjectCard); len(cards) != 2 {
		t.Fatalf("cards: %d, want card-1 and card-2 only", len(cards))
	}

	if _, err := gate.AnswerCard(context.Background(), "p1", "card-2", cycle.OptionKeep, "Porque ajusté la cifra."); !errors.Is(err, cycle.ErrInvalidOption) {
		t.Fatalf("answer with an option: %v, want ErrInvalidOption", err)
	}
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-2", "", "ok"); !errors.Is(err, cycle.ErrInvalidReason) {
		t.Fatalf("answer with ok: %v, want ErrInvalidReason", err)
	}
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-2", "", "Porque el estudio corrigió su cifra."); err != nil {
		t.Fatalf("answer with reason: %v", err)
	}
	card, cb = object(t, db, "card-2")
	if card.State != "answered" || cb["reason"] != "Porque el estudio corrigió su cifra." || cb["chosen_option"] != "" {
		t.Fatalf("card-2 after answer: %s %v", card.State, cb)
	}
	if dec, _ := object(t, db, "dec-3"); dec.State != "confirmed" {
		t.Fatalf("dec-3: %s, want confirmed (unchanged)", dec.State)
	}
	if seen := eventsOfType(t, db, store.EventDecisionSeen); len(seen) != 2 {
		t.Fatalf("decision_seen: %d, want 2 (card-1 and card-2)", len(seen))
	}
	if _, pendings := status(t, db, "sec-1"); hasPending(pendings, graph.PendingOpenCard, "") {
		t.Fatalf("sec-1 pendings after answer: %v", pendings)
	}
}

// B-49: option null is accepted only on the card of your critical edit.
func TestB49_NullOptionIsRefusedOnAnOrdinaryCard(t *testing.T) {
	db := cardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", "", "Porque sí compara igual."); !errors.Is(err, cycle.ErrInvalidOption) {
		t.Fatalf("null option on card-1: %v, want ErrInvalidOption", err)
	}
}

// B-49: an edit that leaves the figure's anchor in place opens nothing.
func TestB49_YourEditAwayFromTheFigureOpensNoCard(t *testing.T) {
	db := cleanCardProject(t)
	_, sb := object(t, db, "sec-1")
	yourEdit(t, db, "sec-1", strings.Replace(sb["text"].(string), "Los estudios usan filtros.", "Los estudios usan filtros finos.", 1))
	if got := cardsOf(t, db); len(got) != 1 {
		t.Fatalf("cards: %v, want only card-1", got)
	}
}

// T-5, fourth reason: marking a decision makes it critical and opens its
// card when it is the agent's; unmarking records marked_critical too.
func TestT5_MarkingADecisionMakesItCriticalAndUnmarkingIsRecorded(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if critical, _, _ := graph.Critical(ctx, db, "p1", "dec-2"); critical {
		t.Fatal("setup: dec-2 is already critical")
	}
	if err := cycle.MarkCritical(ctx, db, "p1", "dec-2", true); err != nil {
		t.Fatalf("mark: %v", err)
	}
	critical, reasons, _ := graph.Critical(ctx, db, "p1", "dec-2")
	if !critical || reasons[0] != graph.ReasonHumanMarked {
		t.Fatalf("dec-2: %v %v", critical, reasons)
	}
	if got := cardsOf(t, db); got["dec-2"] != "open" {
		t.Fatalf("cards: %v, want dec-2's open", got)
	}
	// Your decision gets no card (T-5).
	if err := cycle.MarkCritical(ctx, db, "p1", "dec-1", true); err != nil {
		t.Fatalf("mark dec-1: %v", err)
	}
	if got := cardsOf(t, db); len(got) != 1 {
		t.Fatalf("cards: %v, want no card for dec-1", got)
	}
	if err := cycle.MarkCritical(ctx, db, "p1", "dec-2", false); err != nil {
		t.Fatalf("unmark: %v", err)
	}
	if critical, _, _ := graph.Critical(ctx, db, "p1", "dec-2"); critical {
		t.Fatal("dec-2 still critical after unmarking")
	}
	marks := eventsOfType(t, db, store.EventMarkedCritical)
	if len(marks) != 3 || eventData(t, marks[2])["label"] != "dec-2" || eventData(t, marks[2])["marked"] != false ||
		eventData(t, marks[0])["marked"] != true || marks[2].Actor != "human" {
		t.Fatalf("marked_critical: %v", marks)
	}
	if err := cycle.MarkCritical(ctx, db, "p1", "sec-1", true); !errors.Is(err, cycle.ErrDecisionNotFound) {
		t.Fatalf("mark a section: %v, want ErrDecisionNotFound", err)
	}
}

// Flujos §1 («sin bucles»): no rule stops the chain of steps; every step
// below leaves a pending (rules 0 and 2) and the chain still runs to its
// limit.
func TestB42_NoRuleStopsTheChainOfSteps(t *testing.T) {
	step := `{"stretch":"direct","triage_line":"Vía directa.","closing_risk":"none","note":"n",
		"proposed_roles":[{"section_id":"new-1","role":"conclusion","title":"Conclusiones"}],
		"new_text":[{"section_id":"new-1","base_version":0,"anchor":"","claim_origin":"material","operation":"insert_after","text":"La embotellada trae 3 veces más."}]}`
	db, runner, cap, pub := newCaptureRunner(t, textFixture(t, step))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != cycle.MaxChainSteps {
		t.Fatalf("provider calls: %d, want %d", len(cap.sent), cycle.MaxChainSteps)
	}
	for i, data := range stepEndedData(t, db) {
		if data["stretch"] != "build" || len(anyStrings(data["text_without_decision"])) != 1 {
			t.Fatalf("step %d: %v", i+1, data)
		}
	}
	if step := pub.step(t); step["stop_reason"] != "step_limit" || step["stretch"] != "build" {
		t.Fatalf("published step: %v", step)
	}
}

// R-1 (stored from piece 2.1 on): each new text keeps its claim_origin
// in the step's edit event, in the order of its texts.
func TestR1_StepStoresTheClaimOriginOfEachText(t *testing.T) {
	db := seedStepProject(t)
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",
		"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"model_knowledge","operation":"replace","text":"Fin."}]}`
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied {
		t.Fatalf("ApplyStep: %v", applied.Errors)
	}
	edits := eventsOfType(t, db, store.EventEdit)
	if got := anyStrings(eventData(t, edits[len(edits)-1])["claim_origins"]); len(got) != 1 || got[0] != "model_knowledge" {
		t.Fatalf("claim_origins: %v", got)
	}
}
