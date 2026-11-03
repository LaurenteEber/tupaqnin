package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 2.4 (backlog-r2): «por aclarar». It is born from the
// principal's proposed_open_questions or from the checker (B-44), with
// its blocks edges; it is a pending of what depends on it; you answer
// with an option and a valid «porque».

// proposedQuestion is a proposed_open_questions entry on sec-1, with
// two labelled variants.
const proposedQuestion = `"proposed_open_questions":[{"id":"new-1",` +
	`"question":"¿El piloto D1 cuenta como estudio de campo?",` +
	`"options":[{"option":"Cuenta","contrast":"Agua de una planta real.","consequence":"Entra en la síntesis.","risk":"Se mezcla laboratorio con campo."},` +
	`{"option":"No cuenta","contrast":"Medido en laboratorio.","consequence":"Queda fuera.","risk":"Se pierde un dato."}],` +
	`"depends_on":["sec-1"],"variants":["Con D1 como campo","Sin D1"]}]`

// proposeQuestion applies a step that only proposes the open question.
func proposeQuestion(t *testing.T, db *store.DB) cycle.ApplyResult {
	t.Helper()
	out := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",` + proposedQuestion + `}`
	applied := applyRecorded(t, db, json.RawMessage(out))
	if !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	return applied
}

func intp(n int) *int { return &n }

// currentBlocks are the current blocks edges of an open question, by the
// label of what they block.
func currentBlocks(t *testing.T, db *store.DB, oqLabel string) []string {
	t.Helper()
	oq, _ := object(t, db, oqLabel)
	edges, err := db.EdgesFrom(context.Background(), "p1", oq.ID)
	if err != nil {
		t.Fatalf("EdgesFrom: %v", err)
	}
	var out []string
	for _, e := range edges {
		if e.Type != store.EdgeBlocks {
			continue
		}
		to, err := db.GetObject(context.Background(), "p1", e.ToID)
		if err != nil {
			t.Fatalf("Object: %v", err)
		}
		out = append(out, to.Label)
	}
	sort.Strings(out)
	return out
}

// P4, D21: a proposed open question with depends_on a section opens
// oq-1 with its blocks edge; the section stays draft with the pending
// and its signoff is blocked, listing it.
func TestP4_ProposedOpenQuestionLeavesTheSectionDraftAndBlocksItsSignoff(t *testing.T) {
	db := seedStepProject(t)
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusReady {
		t.Fatalf("before: %s %v, want ready", st, pendings)
	}
	applied := proposeQuestion(t, db)
	if applied.Labels["new-1"] != "oq-1" {
		t.Fatalf("labels: %v, want new-1 → oq-1", applied.Labels)
	}
	oq, body := object(t, db, "oq-1")
	if oq.State != "open" || body["question"] != "¿El piloto D1 cuenta como estudio de campo?" ||
		body["undeclared_by_agent"] != false {
		t.Fatalf("oq-1: %s %v", oq.State, body)
	}
	if opts, _ := body["options"].([]any); len(opts) != 2 {
		t.Fatalf("oq-1 options: %v, want the principal's two", body["options"])
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 1 || got[0] != "sec-1" {
		t.Fatalf("blocks: %v, want sec-1", got)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || !hasPending(pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("after: %s %v, want draft with open_question oq-1", st, pendings)
	}
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by oq-1", open, err)
	}
	// It is applied: no longer kept in step_ended as not applied (B-17).
	if na, _ := lastStepEnded(t, db)["not_applied"].(map[string]any); na["proposed_open_questions"] != nil {
		t.Fatalf("not_applied: %v", na)
	}
}

// Estados §4, fichas §8.2: «B, como dije» without choosing and «ok»
// leave it open; each answer records act_timed, and «ok» records
// answer_without_reason.
func TestE4x4_AnswerWithoutChoosingOrWithAnAssentLeavesItOpen(t *testing.T) {
	db := seedStepProject(t)
	proposeQuestion(t, db)
	gate := cycle.NewGate(db)
	ctx := context.Background()

	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", nil, "B, como dije"); !errors.Is(err, cycle.ErrNoOption) {
		t.Fatalf("without choosing: %v, want ErrNoOption", err)
	}
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(1), "ok"); !errors.Is(err, cycle.ErrInvalidReason) {
		t.Fatalf("ok: %v, want ErrInvalidReason", err)
	}
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque sí lo es en planta."); !errors.Is(err, cycle.ErrInvalidOption) {
		t.Fatalf("option 2 of 2: %v, want ErrInvalidOption", err)
	}
	if oq, _ := object(t, db, "oq-1"); oq.State != "open" {
		t.Fatalf("oq-1: %s, want open", oq.State)
	}
	if _, pendings := status(t, db, "sec-1"); !hasPending(pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("pendings: %v, want oq-1", pendings)
	}
	without := eventsOfType(t, db, store.EventAnswerWithoutReason)
	if len(without) != 1 || eventData(t, without[0])["label"] != "oq-1" {
		t.Fatalf("answer_without_reason: %d, want 1 for «ok»", len(without))
	}
	timed := eventsOfType(t, db, store.EventActTimed)
	if len(timed) != 2 || eventData(t, timed[0])["act"] != "open_question" {
		t.Fatalf("act_timed: %d, want 2 (each answer)", len(timed))
	}
}

// Estados §4: choosing with a «porque» resolves it: the unchosen
// variants go to the history as discarded, the blocks edges end and the
// section is back to draft without the pending (here ready: nothing
// else is pending). The app creates no decision; the principal reads
// the resolved question, with the option chosen, in what changed.
func TestE4x4_ChoosingWithAReasonResolvesItEndsBlocksAndTheSectionIsBack(t *testing.T) {
	db := seedStepProject(t)
	proposeQuestion(t, db)
	decisionsBefore, _ := db.ListObjects(context.Background(), "p1", store.ObjectDecision)

	answer, err := cycle.NewGate(db).AnswerOpenQuestion(context.Background(), "p1", "oq-1", intp(1),
		"Porque se midió en laboratorio, no en grifos.")
	if err != nil || answer.OpenQuestion != "oq-1" || answer.NewDecision != "" {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	oq, body := object(t, db, "oq-1")
	if oq.State != "resolved" || body["chosen_option"] != float64(1) || body["reason"] != "Porque se midió en laboratorio, no en grifos." {
		t.Fatalf("oq-1: %s %v", oq.State, body)
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 0 {
		t.Fatalf("blocks after resolving: %v, want none current", got)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusReady || hasPending(pendings, graph.PendingOpenQuestion, "") {
		t.Fatalf("sec-1: %s %v, want ready without the open question", st, pendings)
	}
	var resolved map[string]any
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		if len(ev.Data) == 0 {
			continue
		}
		if d := eventData(t, ev); d["label"] == "oq-1" && d["to"] == "resolved" {
			resolved = d
			if ev.Reason != "Porque se midió en laboratorio, no en grifos." || ev.Actor != "human" {
				t.Fatalf("resolved event: actor %q reason %q", ev.Actor, ev.Reason)
			}
		}
	}
	if discarded, _ := resolved["discarded_variants"].([]any); len(discarded) != 2 {
		t.Fatalf("resolved event: %v, want the two variants discarded", resolved)
	}
	if len(eventsOfType(t, db, store.EventActTimed)) != 1 {
		t.Fatal("act_timed: want one for the answer")
	}
	if decisions, _ := db.ListObjects(context.Background(), "p1", store.ObjectDecision); len(decisions) != len(decisionsBefore) {
		t.Fatalf("decisions: %d, want %d (the app creates none)", len(decisions), len(decisionsBefore))
	}
	changes, err := memory.New(db).ChangesSinceDelivery(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ChangesSinceDelivery: %v", err)
	}
	found := false
	for _, c := range changes {
		found = found || c.Label == "oq-1" && c.State == "resolved" && strings.Contains(c.FirstLine, "No cuenta")
	}
	if !found {
		t.Fatalf("what changed: %+v, want oq-1 resolved with the chosen option", changes)
	}
	if _, err := cycle.NewGate(db).AnswerOpenQuestion(context.Background(), "p1", "oq-1", intp(0), "Porque cambié de idea."); !errors.Is(err, cycle.ErrOpenQuestionNotOpen) {
		t.Fatalf("answer again: %v, want ErrOpenQuestionNotOpen", err)
	}
}

// Estados §1: opening a «por aclarar» on a signed section moves it to
// changed_since_signoff.
func TestE1_OpenQuestionOnASignedSectionMovesItToChangedSinceSignoff(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		_, err = tx.UpdateObject(sec.ID, sec.CurrentVersion, "signed", b,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	proposeQuestion(t, db)
	if st := sectionState(t, db, "sec-1"); st != "changed_since_signoff" {
		t.Fatalf("sec-1: %s, want changed_since_signoff", st)
	}
	// Resolving it does not sign it again (estados §4: it stays in «cambió»).
	if _, err := cycle.NewGate(db).AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), "Porque se tomó en una planta real."); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if st := sectionState(t, db, "sec-1"); st != "changed_since_signoff" {
		t.Fatalf("sec-1 after resolving: %s, want changed_since_signoff", st)
	}
}

// Estados §6, P4: the checker's contradiction with the premise moves it
// to contradicted; the app offers «mantener» first; changing it creates
// the premise's new version (the chosen option) and probes the current
// text against it (B-46).
func TestE6_ChangingTheContradictedPremiseCreatesItsNewVersionAndProbesTheCurrentText(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"), checkerFixture(t, "no-findings"))
	ctx := context.Background()
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	prem, _ := object(t, db, "prem-1")
	if prem.State != "contradicted" {
		t.Fatalf("prem-1: %s, want contradicted", prem.State)
	}
	_, body := object(t, db, "oq-1")
	opts, _ := body["options"].([]any)
	if len(opts) < 3 {
		t.Fatalf("oq-1 options: %v, want keep plus the checker's two or more (B-44, P-71)", opts)
	}
	keep, _ := opts[0].(map[string]any)
	if keep["keep"] != true || keep["option"] != checkerPremise {
		t.Fatalf("first option: %v, want to keep the premise", keep)
	}

	answer, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), "Porque los muestreos nuevos no muestran diferencia.")
	if err != nil || !answer.PremiseChanged {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	prem, pb := object(t, db, "prem-1")
	want := chosenOption(t, db, "oq-1", 2)
	versions, err := db.Versions(ctx, "p1", prem.ID)
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if prem.State != "current" || pb["text"] != want || len(versions) != 3 {
		t.Fatalf("prem-1: %s v%d %v (%d versions), want current with the chosen option as a new version",
			prem.State, prem.CurrentVersion, pb, len(versions))
	}
	if len(cap.checker) != 2 {
		t.Fatalf("checker calls: %d, want 2 (the batch, then the probe of the new premise)", len(cap.checker))
	}
	brief := briefOf(t, cap.checker[1])
	for _, s := range []string{"prem-1 (premise): " + want, "New passage of section sec-1", contradictingEdit} {
		if !strings.Contains(brief, s) {
			t.Fatalf("probe brief lacks %q:\n%s", s, brief)
		}
	}
}

// Estados §6: keeping the contradicted premise returns it to current,
// with its text, and probes nothing.
func TestE6_KeepingTheContradictedPremiseReturnsItToCurrent(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, cap := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"))
	ctx := context.Background()
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	answer, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), "Porque la tesis se sostiene con el informe.")
	if err != nil || answer.PremiseChanged {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	if prem, pb := object(t, db, "prem-1"); prem.State != "current" || pb["text"] != checkerPremise {
		t.Fatalf("prem-1: %s %v, want current with its text", prem.State, pb)
	}
	if len(cap.checker) != 1 {
		t.Fatalf("checker calls: %d, want 1 (keeping probes nothing)", len(cap.checker))
	}
}

// Estados §2 (as B-16): a «por aclarar» that blocks a decision of yours
// offers «mantener: <enunciado>» first, the decision does not change
// state while it is open, and choosing another option replaces it with
// a decision of yours: the option as its statement, your «porque».
func TestE2_OpenQuestionOnYourDecisionKeepsItFirstAndAnotherOptionReplacesIt(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", "La síntesis compara estudios. Usamos solo datos de tesis, no el informe del comité.")
	runner, _ := checkerRunner(t, db, checkerFixture(t, "contradicts-decision"))
	ctx := context.Background()
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 2 || got[0] != "dec-1" || got[1] != "sec-1" {
		t.Fatalf("blocks: %v, want sec-1 and dec-1", got)
	}
	if dec, _ := object(t, db, "dec-1"); dec.State != "human" {
		t.Fatalf("dec-1: %s, want human (blocking does not move it)", dec.State)
	}
	_, body := object(t, db, "oq-1")
	opts, _ := body["options"].([]any)
	if keep, _ := opts[0].(map[string]any); keep["keep"] != true || keep["option"] != "Usar el informe del comité." {
		t.Fatalf("first option: %v, want to keep dec-1", opts[0])
	}
	reason := "Porque el comité no publicó datos de campo."
	answer, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(2), reason)
	if err != nil || answer.NewDecision != "dec-3" {
		t.Fatalf("answer: %+v, %v", answer, err)
	}
	cycle.WaitTurnsForTest(runner) // B-100: the step it started
	if dec, _ := object(t, db, "dec-1"); dec.State != "superseded" {
		t.Fatalf("dec-1: %s, want superseded", dec.State)
	}
	// The born decision says the chosen option, whatever the checker
	// wrote (a recording from Meta words it otherwise, P-71).
	chosen, _ := opts[2].(map[string]any)
	dec, db3 := object(t, db, "dec-3")
	if dec.State != "human" || db3["statement"] != chosen["option"] ||
		db3["instead_of"] != "Usar el informe del comité." || db3["reason"] != reason || db3["origin"] != "human" {
		t.Fatalf("dec-3: %s %v", dec.State, db3)
	}
	if got := edgeTypes(t, db, dec.ID)[store.EdgeSupersedes]; len(got) != 1 {
		t.Fatalf("supersedes: %v", got)
	}
}

// P5, D21: with a «por aclarar» open the agent keeps chaining steps.
func TestP5_WithAnOpenQuestionOpenTheAgentKeepsChainingSteps(t *testing.T) {
	db := seedStepProject(t)
	// A section still to write keeps work for the agent (backlog-r2 2.5:
	// with only the «por aclarar» left, the chain would stop).
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Cierre", "text": ""},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed sec-2: %v", err)
	}
	second := `{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",` +
		`"new_text":[{"section_id":"sec-1","base_version":2,"anchor":"Texto uno.","claim_origin":"material","operation":"replace","text":"Texto dos."}]}`
	runCheckedTurn(t, db, nil, stepWriting(t, "Texto uno.", proposedQuestion+","), textFixture(t, second))

	if oq, _ := object(t, db, "oq-1"); oq.State != "open" {
		t.Fatalf("oq-1: %s, want open", oq.State)
	}
	if n := len(stepEndedData(t, db)); n != 3 {
		t.Fatalf("step_ended: %d, want 3 (the chain went on past the open question)", n)
	}
	if text := bodyText(t, db, "sec-1"); !strings.Contains(text, "Texto dos.") {
		t.Fatalf("sec-1: %q, want the second step's text", text)
	}
}

// P-67: asking to sign after the checker opened a «por aclarar» lists it
// as the signoff's pending, and the signoff does not open.
func TestP67_AskingToSignAfterTheCheckerOpenedAnOpenQuestionListsItAsPending(t *testing.T) {
	db := seedCheckerProject(t)
	yourEdit(t, db, "sec-1", contradictingEdit)
	runner, _ := checkerRunner(t, db, checkerFixture(t, "contradicts-premise"))
	open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by oq-1", open, err)
	}
}
