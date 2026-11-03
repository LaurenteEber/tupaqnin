package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-114: the gaps of slice 4 Eber decided on 2-nov.

func hasKind(pendings []graph.Pending, kind string) bool {
	for _, p := range pendings {
		if p.Kind == kind {
			return true
		}
	}
	return false
}

// proposeOnSigned leaves a proposal on sec-1, signed, by a step, and returns it.
func proposeOnSigned(t *testing.T, db *store.DB) string {
	t.Helper()
	setSectionSigned(t, db, "sec-1")
	applied, err := cycle.NewGate(db).ApplyStep(context.Background(), "p1", "turn-x", json.RawMessage(onSigned))
	if err != nil || len(applied.Proposals) != 1 {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	return applied.Proposals[0]
}

// C1: the accepted text is «sin comprobar» until the checker's next pass, it is in
// that pass, and the signoff's own check sees it first.
func TestB50_AnAcceptedProposalStaysUncheckedUntilTheCheckerSeesIt(t *testing.T) {
	db := seedCheckerProject(t)
	pass := textFixture(t, `{"probes": [`+
		`{"protected": "req-1", "verdict": "no", "passage": null}, {"protected": "crit-1", "verdict": "no", "passage": null},`+
		`{"protected": "prem-1", "verdict": "no", "passage": null}, {"protected": "dec-1", "verdict": "no", "passage": null},`+
		`{"protected": "dec-2", "verdict": "no", "passage": null}], "candidates": []}`)
	pass.TurnKind, pass.Deltas = provider.TurnChecker, nil
	runner, cap := checkerRunner(t, db, pass)
	t.Cleanup(runner.Close)
	ctx := context.Background()
	setSectionSigned(t, db, "sec-1")
	const accepted = "Cierre corregido por el agente."
	applied, err := cycle.NewGate(db).ApplyStep(ctx, "p1", "turn-x", json.RawMessage(`{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"n",`+
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Cierre.","claim_origin":"material","operation":"replace","text":"`+accepted+`"}]}`))
	if err != nil || len(applied.Proposals) != 1 {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	label := applied.Proposals[0]
	if _, pendings := status(t, db, "sec-1"); hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("unchecked before accepting: %v", pendings)
	}
	if out, err := cycle.NewGate(db).AcceptProposal(ctx, "p1", label); err != nil || out.State != "accepted" {
		t.Fatalf("AcceptProposal: %+v, %v", out, err)
	}
	if _, pendings := status(t, db, "sec-1"); !hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("the accepted text is not «sin comprobar»: %v", pendings)
	}
	edits, err := checks.YourEditsToCheck(ctx, db, "p1")
	if err != nil || len(edits.Sections) != 1 {
		t.Fatalf("the next batch lacks the accepted text: %+v, %v", edits, err)
	}
	if _, err := runner.RequestSignoff(ctx, "p1", "sec-1"); err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if len(cap.checker) != 1 || !strings.Contains(itemsText(cap.checker[0].Input), accepted) {
		t.Fatalf("the checker did not see the accepted text: %d calls", len(cap.checker))
	}
	if _, pendings := status(t, db, "sec-1"); hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("still unchecked after its pass: %v", pendings)
	}
}

// C1: rule 2 on a conclusion section reads the same as for a step: accepted text
// with no decision is «escrito sin decisión»; one that declared a decision is not.
func TestB42_AnAcceptedTextWithoutDecisionInAConclusionIsAPending(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	label := proposeOnSigned(t, db)
	if _, err := cycle.NewGate(db).AcceptProposal(context.Background(), "p1", label); err != nil {
		t.Fatal(err)
	}
	if _, pendings := status(t, db, "sec-1"); !hasKind(pendings, graph.PendingTextWithoutDecision) {
		t.Fatalf("pendings: %v, want text_without_decision", pendings)
	}
}

// C1: rule 1 reads the accepted text as any: a figure with no decision is a pending,
// and a discarded proposal leaves no «sin comprobar» behind.
func TestB50_ADiscardedProposalLeavesNothingUnchecked(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	label := proposeOnSigned(t, db)
	if _, err := cycle.NewGate(db).DiscardProposal(context.Background(), "p1", label); err != nil {
		t.Fatal(err)
	}
	if _, pendings := status(t, db, "sec-1"); hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("pendings: %v", pendings)
	}
}

// C2: over the threshold with a signoff in progress your message is queued with its
// line, no step runs, and the signoff's close answers it, outside the mode too.
func TestB86_YourMessageDuringASignoffOverTheThresholdWaitsAndTheCloseAnswersIt(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	gate := cycle.NewGate(db)
	sign := openSignoff(t, gate)
	runner, cap, _ := b86Runner(t, db, nil, using(textFixture(t, pendingFigureStep), 7000), b86NoWork(t, 100))
	t.Cleanup(runner.Close)
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Escribe."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("steps before your message: %d", len(cap.sent))
	}
	const msg = "¿Qué falta en la regla?"
	turn, err := runner.StartTurn("p1", msg)
	if err != nil || turn != "" {
		t.Fatalf("StartTurn: %q, %v; want it queued", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("a step ran during the signoff: %d", len(cap.sent))
	}
	messages := eventsOfType(t, db, store.EventHumanMessage)
	var data map[string]any
	_ = json.Unmarshal(messages[len(messages)-1].Data, &data)
	if data["text"] != msg || data["queued"] != true || data["waits_signoff"] != true {
		t.Fatalf("your message: %v", data)
	}
	// The signoff's close answers it: it waits, and nothing says it was not sent.
	if queued, err := runner.QueuedMessages(ctx, "p1"); err != nil || len(queued) != 1 || queued[0].NotSent {
		t.Fatalf("during the signoff: %+v %v", queued, err)
	}
	if err := gate.LeaveSignoff(ctx, "p1", sign); err != nil {
		t.Fatal(err)
	}
	runner.SignoffClosed(ctx, "p1")
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 2 || !strings.Contains(itemsText(cap.sent[1]), msg) {
		t.Fatalf("the close did not answer it: %d steps", len(cap.sent))
	}
}

// C2: what it touches: below the threshold a signoff does not queue your message.
func TestB86_BelowTheThresholdYourMessageDuringASignoffStartsATurn(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	openSignoff(t, cycle.NewGate(db))
	runner, cap, _ := b86Runner(t, db, nil, b86NoWork(t, 100))
	t.Cleanup(runner.Close)
	turn, err := runner.StartTurn("p1", "Hola.")
	if err != nil || turn == "" {
		t.Fatalf("StartTurn: %q, %v", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("steps: %d", len(cap.sent))
	}
}

// C3: a mode a restart left on ends: mode_finished with cause restart, no model
// call, mode_set back to normal, the return line, once.
func TestB85_ARestartEndsAModeLeftRunningWithItsReturnLine(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 2}
	db := openTemp(t)
	runner, _, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, "Termina."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	runner.Close()
	if got := currentMode(t, db); got != graph.ModeRunToCompletion {
		t.Fatalf("mode after Close: %s", got)
	}
	runner2, cap2, _ := modeRunnerOn(t, db, nil)
	ctx := context.Background()
	for range 2 {
		if err := runner2.EndModesLeftRunning(ctx); err != nil {
			t.Fatal(err)
		}
	}
	report := oneReport(t, db)
	if report.StopReason != cycle.StopModeFinished || report.Cause != "restart" {
		t.Fatalf("report: %s %s", report.StopReason, report.Cause)
	}
	if got := currentMode(t, db); got != graph.ModeNormal {
		t.Fatalf("mode: %s", got)
	}
	sets := eventsOfType(t, db, store.EventModeSet)
	var data map[string]any
	_ = json.Unmarshal(sets[len(sets)-1].Data, &data)
	if sets[len(sets)-1].Actor != "app" || data["cause"] != "restart" {
		t.Fatalf("last mode_set: %v", data)
	}
	if len(cap2.sent) != 0 || len(cap2.reports) != 0 {
		t.Fatalf("model calls: %d steps, %d reports", len(cap2.sent), len(cap2.reports))
	}
	frame, err := graph.ComputeFrame(ctx, db, "p1", graph.FrameLayers{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if frame.ReturnLine == nil || !frame.ReturnLine.ModeFinished {
		t.Fatalf("return line: %+v", frame.ReturnLine)
	}
}

// C3: what it touches: a project in normal, or never in the mode, gets nothing.
func TestB85_TheRestartSweepLeavesANormalModeAlone(t *testing.T) {
	db, runner, _, _ := newModeRunner(t, nil, loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, false)
	before := len(allEvents(t, db))
	if err := runner.EndModesLeftRunning(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(allEvents(t, db)); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
}

// C4: three steps in a row that only ask end the mode with no_work.
func TestB83_ThreeStepsThatOnlyAskEndTheMode(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	runner, cap, pub := modeRunnerOn(t, db, nil, textFixture(t, proposalOpenQuestionStep))
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	// Your message's step does not count, so three more follow it.
	if len(cap.sent) != 1+cycle.ModeMaxIdleSteps {
		t.Fatalf("steps: %d, want %d", len(cap.sent), 1+cycle.ModeMaxIdleSteps)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// C4: work in between resets the count.
func TestB83_WorkBetweenStepsThatOnlyAskResetsTheCount(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	ask := func() provider.Fixture { return textFixture(t, proposalOpenQuestionStep) }
	runner, cap, pub := modeRunnerOn(t, db, nil, ask(), ask(), ask(), textFixture(t, pendingFigureStep), ask(), ask(), ask())
	modeOn(t, db, "")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 7 {
		t.Fatalf("steps: %d, want 7", len(cap.sent))
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// C5: an objection opened after the pendings were first read still stops the
// signature: Sign reads them inside its transaction. StoppedBy runs between that
// read and the transaction in the code this fixes, so the test is deterministic.
func TestE1_SignReadsThePendingsInsideItsTransaction(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	open, err := gate.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v, %v", open, err)
	}
	if err := gate.RecordExplanation(ctx, "p1", open.Label, "Lo explico yo."); err != nil {
		t.Fatal(err)
	}
	coincideOnThesis(t, db, gate, open.Label) // its decisions are not critical (B-106)
	gate.StoppedBy = func(ctx context.Context, projectID string) (string, error) {
		_, err := cycle.NewGate(db).ObjectDecision(ctx, projectID, "dec-2", objectReason)
		return "", err
	}
	var blocked *cycle.PendingsError
	if err := gate.Sign(ctx, "p1", open.Label); !errors.As(err, &blocked) || !hasKind(blocked.Pendings, graph.PendingOpenQuestion) {
		t.Fatalf("Sign: %v, want the objection among its pendings", err)
	}
	if st := sectionState(t, db, "sec-1"); st == "signed" {
		t.Fatal("signed over an objection")
	}
}

// appendProbe is a checker's pass over sec-1, as far as the history says.
func appendProbe(t *testing.T, db *store.DB, data map[string]any) {
	t.Helper()
	data["sections"] = []string{"sec-1"}
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventProbe, Actor: "agent", Data: data})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// C1: only a pass that read the accepted version clears it: not a step's pass, which
// gets that step's passage, and not a batch that read the section before you accepted.
func TestB50_OnlyAPassThatReadTheAcceptedTextClearsIt(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	ctx := context.Background()
	label := proposeOnSigned(t, db)
	if _, err := cycle.NewGate(db).AcceptProposal(ctx, "p1", label); err != nil {
		t.Fatal(err)
	}
	version := func() int {
		sec, err := db.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			t.Fatal(err)
		}
		return sec.CurrentVersion
	}
	stillOpen := func(when string) {
		t.Helper()
		if _, pendings := status(t, db, "sec-1"); !hasKind(pendings, graph.PendingUnchecked) {
			t.Fatalf("%s: it reads as checked: %v", when, pendings)
		}
		if edits, err := checks.YourEditsToCheck(ctx, db, "p1"); err != nil || len(edits.Sections) != 1 {
			t.Fatalf("%s: the next batch lacks it: %+v, %v", when, edits, err)
		}
	}
	appendProbe(t, db, map[string]any{"kind": "probe"})
	stillOpen("after a step's pass")
	appendProbe(t, db, map[string]any{"kind": "probe", "saw_versions": map[string]int{"sec-1": version() - 1}})
	stillOpen("after a batch that read the version before")
	appendProbe(t, db, map[string]any{"kind": "probe", "saw_versions": map[string]int{"sec-1": version()}})
	if _, pendings := status(t, db, "sec-1"); hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("after the pass that read it: %v", pendings)
	}
	if edits, err := checks.YourEditsToCheck(ctx, db, "p1"); err != nil || len(edits.Sections) != 0 {
		t.Fatalf("the batch goes on asking: %+v, %v", edits, err)
	}
}

// C1: a pass that fails leaves it unchecked and the next trigger sends it again
// (P-74), so it cannot stay blocked with nothing left to clear it.
func TestB50_AFailedPassOverAnAcceptedTextIsRetriedAtTheNextTrigger(t *testing.T) {
	db := openTemp(t)
	seedSignable(t, db, false)
	label := proposeOnSigned(t, db)
	ctx := context.Background()
	if _, err := cycle.NewGate(db).AcceptProposal(ctx, "p1", label); err != nil {
		t.Fatal(err)
	}
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventOutputRetry, Actor: "app",
			Data: map[string]any{"turn_kind": "checker", "result": "unchecked", "sections": []string{"sec-1"}}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if edits, err := checks.YourEditsToCheck(ctx, db, "p1"); err != nil || len(edits.Sections) != 1 {
		t.Fatalf("the next trigger does not send it: %+v, %v", edits, err)
	}
}

// C4: a step that leaves a proposal is not idle, even with no text of its own: a new
// section declared after the plan (B-77), together with a question.
func TestB83_AStepThatLeavesAProposalDoesNotCountAsIdle(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if _, err := cycle.ConfirmPlan(ctx, db, "p1"); err != nil {
		t.Fatal(err)
	}
	var fixtures []provider.Fixture
	for _, title := range []string{"Una", "Dos", "Tres", "Cuatro"} {
		fixtures = append(fixtures, textFixture(t, `{"stretch":"build","triage_line":"C.","closing_risk":"none","note":"n",`+proposedQuestion+
			`,"proposed_roles":[{"section_id":"new-9","role":"other","title":"`+title+`"}]}`))
	}
	// The fake replays its list in a loop, so the idle steps are listed too.
	for range cycle.ModeMaxIdleSteps {
		fixtures = append(fixtures, textFixture(t, proposalOpenQuestionStep))
	}
	runner, cap, pub := modeRunnerOn(t, db, nil, fixtures...)
	modeOn(t, db, "")
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatal(err)
	}
	if len(cap.sent) != 4+cycle.ModeMaxIdleSteps {
		t.Fatalf("steps: %d, want %d", len(cap.sent), 4+cycle.ModeMaxIdleSteps)
	}
	wantModeFinished(t, db, pub, "no_work", "")
}

// C1: a section you accepted into being (B-77) waits for its pass too.
func TestB50_AnAcceptedNewSectionStaysUnchecked(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if _, err := cycle.ConfirmPlan(ctx, db, "p1"); err != nil {
		t.Fatal(err)
	}
	applied, err := cycle.NewGate(db).ApplyStep(ctx, "p1", "turn-x", json.RawMessage(`{"stretch":"build","triage_line":"C.","closing_risk":"none","note":"n",`+
		`"new_text":[{"section_id":"new-9","base_version":0,"anchor":"","claim_origin":"material","operation":"insert_after","text":"Sección nueva."}],`+
		`"proposed_roles":[{"section_id":"new-9","role":"other","title":"Otra"}]}`))
	if err != nil || len(applied.Proposals) != 1 {
		t.Fatalf("ApplyStep: %+v, %v", applied, err)
	}
	out, err := cycle.NewGate(db).AcceptProposal(ctx, "p1", applied.Proposals[0])
	if err != nil || out.State != "accepted" {
		t.Fatalf("AcceptProposal: %+v, %v", out, err)
	}
	if _, pendings := status(t, db, out.Section); !hasKind(pendings, graph.PendingUnchecked) {
		t.Fatalf("%s: %v", out.Section, pendings)
	}
}
