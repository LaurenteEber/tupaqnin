package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const applyReason = "Porque el comité pidió conteos."

// closesChange is the step that applies your decision and closes what it opened.
func closesChange(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Aplico dec-4.", "closing_risk": "none", "note": "Apliqué tu decisión.",
		"handled_changes": []any{map[string]any{"change_id": "chg-1", "result": "reviewed_unchanged", "reason": "El texto ya no depende de lo anterior."}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// B-100: «la otra» with the section clear runs one step, whose task names the
// decision, and the section stops being «escrita con lo anterior».
func TestB100_TheOtherOptionWithTheSectionClearRunsOneStep(t *testing.T) {
	db := cleanCardProject(t)
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, closesChange(t)), loadAppFixture(t, "04-principal-no-work.json"))
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason)
	if err != nil || answer.TurnID == "" {
		t.Fatalf("AnswerCard: %+v %v, want a turn", answer, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("step calls: %d, want one step, not a chain", len(cap.sent))
	}
	task := lastUserText(t, cap.sent[0])
	if !strings.Contains(task, "dec-4") || !strings.Contains(task, "sec-1") {
		t.Fatalf("the step's task: %q", task)
	}
	if st, _ := status(t, db, "sec-1"); st == graph.StatusStale {
		t.Fatalf("sec-1 is still «escrita con lo anterior»")
	}
	if left, err := graph.UnappliedDecisions(context.Background(), db, "p1"); err != nil || len(left) != 0 {
		t.Fatalf("unapplied after the step: %+v %v", left, err)
	}
}

// B-100: the step's new text reaches the section: your decision is in it, its
// change is closed, and it stops being «escrita con lo anterior».
func TestB100_TheStepUpdatesTheTextWithYourDecision(t *testing.T) {
	db := cleanCardProject(t)
	sec, _ := object(t, db, "sec-1")
	const updated = "Se excluye el piloto de laboratorio de la comparación."
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Aplico dec-4: excluir el piloto.", "closing_risk": "none", "note": "Apliqué tu decisión.",
		"new_text": []any{map[string]any{"section_id": "sec-1", "base_version": sec.CurrentVersion + 1, "anchor": "Cierre.", "operation": "replace",
			"claim_origin": "material", "text": updated}},
		"handled_changes": []any{map[string]any{"change_id": "chg-1", "result": "updated"}}})
	if err != nil {
		t.Fatal(err)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, string(raw)), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	_, after := object(t, db, "sec-1")
	if !strings.Contains(after["text"].(string), updated) {
		for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
			t.Logf("retry: %s", ev.Data)
		}
		t.Fatalf("the text: %q", after["text"])
	}
	if st, _ := status(t, db, "sec-1"); st == graph.StatusStale {
		t.Fatal("sec-1 is still «escrita con lo anterior»")
	}
	if left, _ := graph.UnappliedDecisions(context.Background(), db, "p1"); len(left) != 0 {
		t.Fatalf("unapplied: %+v", left)
	}
}

// B-100: with the agent's step not touching the section, it stays listed.
func TestB100_AStepThatDoesNotTouchTheSectionLeavesItListed(t *testing.T) {
	db := cleanCardProject(t)
	runner, _, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	left, err := graph.UnappliedDecisions(context.Background(), db, "p1")
	if err != nil || len(left) != 1 || left[0].Section != "sec-1" || left[0].Decision != "dec-4" || !left[0].Ready {
		t.Fatalf("unapplied: %+v %v", left, err)
	}
}

// B-100: «mantener» runs nothing.
func TestB100_KeepRunsNothing(t *testing.T) {
	db := cleanCardProject(t)
	runner, cap, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara igual.")
	if err != nil || answer.TurnID != "" {
		t.Fatalf("AnswerCard: %+v %v, want no turn", answer, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 0 {
		t.Fatalf("step calls: %d, want none", len(cap.sent))
	}
}

// B-100: with something else of yours open in the section nothing runs; the last
// answer, even a «mantener», runs the one step for what waited.
func TestB100_WithAnotherPendingNothingRunsUntilTheLastAnswer(t *testing.T) {
	db := cleanCardProject(t)
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, closesChange(t)), loadAppFixture(t, "04-principal-no-work.json"))
	objected, err := cycle.NewGate(db).ObjectDecision(context.Background(), "p1", "dec-2", "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	if _, pendings := status(t, db, "sec-1"); len(pendings) < 2 {
		t.Fatalf("setup: pendings %v, want the card and the question", pendings)
	}
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason)
	if err != nil || answer.TurnID != "" {
		t.Fatalf("AnswerCard: %+v %v, want no turn", answer, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 0 {
		t.Fatalf("step calls with the question open: %d", len(cap.sent))
	}
	left, _ := graph.UnappliedDecisions(context.Background(), db, "p1")
	if len(left) != 1 || left[0].Ready {
		t.Fatalf("unapplied: %+v, want one not ready", left)
	}
	keep := 0 // the objection's first option is «mantener»
	last, err := runner.AnswerOpenQuestion(context.Background(), "p1", objected.OpenQuestion, &keep, "Porque la decisión sigue bien.")
	if err != nil || last.TurnID == "" {
		t.Fatalf("AnswerOpenQuestion: %+v %v, want the one step", last, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("step calls: %d, want one", len(cap.sent))
	}
}

// B-100: on a signed section the step leaves a proposal (B-73), and that closes it.
func TestB100_OnASignedSectionTheStepLeavesAProposal(t *testing.T) {
	db := cleanCardProject(t)
	sec, body := object(t, db, "sec-1")
	setSectionSigned(t, db, "sec-1")
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Aplico dec-4.", "closing_risk": "none", "note": "Propongo.",
		"new_text": []any{map[string]any{"section_id": "sec-1", "base_version": sec.CurrentVersion + 1, "anchor": body["text"], "operation": "replace",
			"claim_origin": "model_knowledge", "text": "El texto con tu decisión."}}})
	if err != nil {
		t.Fatal(err)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, string(raw)), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	if after, afterBody := object(t, db, "sec-1"); afterBody["text"] != body["text"] {
		t.Fatalf("a signed section's text changed: %s %v", after.State, afterBody["text"])
	}
	proposals := 0
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		var d map[string]any
		_ = json.Unmarshal(ev.Data, &d)
		if d["section"] == "sec-1" && d["to"] == "open" {
			proposals++
		}
	}
	if proposals != 1 {
		t.Fatalf("proposals for sec-1: %d", proposals)
	}
	if left, _ := graph.UnappliedDecisions(context.Background(), db, "p1"); len(left) != 0 {
		t.Fatalf("unapplied after the proposal: %+v", left)
	}
}

// B-100: with the slot taken it waits, and the release runs the one step.
func TestB100_WithTheTurnBusyItWaitsForTheRelease(t *testing.T) {
	db := cleanCardProject(t)
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, closesChange(t)), loadAppFixture(t, "04-principal-no-work.json"))
	end := cycle.HoldTurnForTest(runner, "p1")
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, applyReason)
	if err != nil || answer.TurnID != "" {
		t.Fatalf("AnswerCard: %+v %v", answer, err)
	}
	if len(cap.sent) != 0 {
		t.Fatalf("a step ran with the slot taken")
	}
	end()
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 || !strings.Contains(lastUserText(t, cap.sent[0]), "dec-4") {
		t.Fatalf("after the release: %d calls", len(cap.sent))
	}
}
