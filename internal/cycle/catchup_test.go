package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	catchUpReasonOne = "Porque el comité pidió conteos, no veces."
	catchUpReasonTwo = "Porque la serie de 2024 es la única comparable."
	correctionText   = "Correct «A figure without a decision · 7» in «Síntesis»."
)

// closeStretch writes a report so what the seed did stays out of the stretch.
func closeStretch(t *testing.T, db *store.DB) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent",
			Data: map[string]any{"stop_reason": "no_work"}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func lastReport(t *testing.T, db *store.DB) cycle.StopReport {
	t.Helper()
	reports := stopReports(t, db)
	if len(reports) == 0 {
		t.Fatal("no stop report")
	}
	return reports[len(reports)-1]
}

// editSection is a step that changes sec-1, on its current version.
func editSection(t *testing.T, db *store.DB, triage, anchor, text string) string {
	t.Helper()
	sec, _ := object(t, db, "sec-1")
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": triage, "closing_risk": "none", "note": "Paso.",
		"new_text": []any{map[string]any{"section_id": "sec-1", "base_version": sec.CurrentVersion, "anchor": anchor,
			"operation": "replace", "claim_origin": "material", "text": text}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// B-103 (a), Listo cuando: a stretch with 2 decisions and 1 step on its own lists
// both with their reason in literal, and the step with its section, its one line
// and what left and entered.
func TestB103_TheReportListsTwoDecisionsWithTheirReasonAndTheStepOnItsOwn(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if _, err := gate.AnswerCard(ctx, "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	objected, err := gate.ObjectDecision(ctx, "p1", "dec-2", "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	other := 1
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", objected.OpenQuestion, &other, catchUpReasonTwo); err != nil {
		t.Fatal(err)
	}
	if applied := applyRecorded(t, db, json.RawMessage(pendingFigureStep)); !applied.Applied {
		t.Fatalf("the step: %v", applied.Errors)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatal(err)
	}

	report := lastReport(t, db)
	reasons := map[string]bool{}
	for _, d := range report.Decided {
		reasons[d.Reason] = d.Who == "human" && d.Statement != "" && len(d.Sections) > 0
	}
	if len(report.Decided) != 2 || !reasons[catchUpReasonOne] || !reasons[catchUpReasonTwo] {
		t.Fatalf("decided: %+v", report.Decided)
	}
	if len(report.AgentChanges) != 1 {
		t.Fatalf("agent changes: %+v", report.AgentChanges)
	}
	change := report.AgentChanges[0]
	if change.Section == "" || change.Step != 1 || change.Summary != "Construir: escribo la muestra." ||
		len(change.Changes) != 1 || !strings.Contains(change.Changes[0].In, "Revisamos 12 estudios") {
		t.Fatalf("the step: %+v", change)
	}
}

// B-103 (a): an agent's decision says who and its «en vez de»; a decision replaced
// inside the stretch is not listed, and «mantener» counts, with your reason.
func TestB103_DecidedSaysWhoAndKeepsYourReasonOnAKeptDecision(t *testing.T) {
	db := cleanCardProject(t) // its step created the agent's decision of the card
	before, err := graph.ReportDecisions(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var agent bool
	for _, d := range before {
		agent = agent || (d.Who == "agent" && d.Statement == "Comparar por veces." && d.InsteadOf == "Comparar por conteos crudos.")
	}
	if !agent {
		t.Fatalf("the agent's decision: %+v", before)
	}
	closeStretch(t, db)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	after, err := graph.ReportDecisions(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Who != "human" || after[0].Reason != catchUpReasonOne || after[0].Statement != "Comparar por veces." {
		t.Fatalf("kept: %+v", after)
	}
}

// B-103, Listo cuando: the step that applied your decision is not on its own.
func TestB103_AStepThatAppliedYourDecisionIsNotOnItsOwn(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	step := editSection(t, db, "Aplico tu decisión.", "Cierre.", "Se excluye el piloto.")
	// The answer creates the decision, and the section moves a version with its change.
	sec, _ := object(t, db, "sec-1")
	step = strings.Replace(step, `"base_version":`+jsonInt(sec.CurrentVersion), `"base_version":`+jsonInt(sec.CurrentVersion+1), 1)
	step = strings.Replace(step, `"note":"Paso."`, `"note":"Paso.","handled_changes":[{"change_id":"chg-1","result":"updated"}]`, 1)
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	report := lastReport(t, db)
	if len(report.Decided) != 1 || report.Decided[0].Reason != catchUpReasonOne {
		t.Fatalf("decided: %+v", report.Decided)
	}
	if len(report.AgentChanges) != 0 {
		t.Fatalf("the step that applied your decision is listed: %+v", report.AgentChanges)
	}
}

func jsonInt(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

// B-103, with unapplied.go's reading: any step that reaches the section your answer
// waited on applied it, even one of a chain that was not asked to; the same step's
// other section is still on its own.
func TestB103_ThePlainStepReachingYourAnswersSectionIsNotOnItsOwn(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	sec, _ := object(t, db, "sec-1")
	step := strings.Replace(string(stepFile(t, "new-section-and-edit")), `"base_version": 1`, `"base_version": `+jsonInt(sec.CurrentVersion), 1)
	if applied := applyRecorded(t, db, json.RawMessage(step)); !applied.Applied {
		t.Fatalf("the step: %v", applied.Errors)
	}
	changes, err := graph.ReportAgentChanges(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Section == "sec-1" {
		t.Fatalf("changes: %+v, want only the other section", changes)
	}
}

// B-100's applies_during: the step in flight when you answered did not read it, so
// what it wrote is on its own; the answer stays to apply.
func TestB103_TheStepInFlightWhenYouAnsweredIsOnItsOwn(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	sec, _ := object(t, db, "sec-1")
	step := strings.Replace(editSection(t, db, "Corrijo el cierre.", "Cierre.", "Cierre revisado."),
		`"base_version":`+jsonInt(sec.CurrentVersion), `"base_version":`+jsonInt(sec.CurrentVersion+1), 1)
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	runner, _, _ := modeRunnerOn(t, db, hold, textFixture(t, step), noWork, noWork, noWork, noWork, noWork)
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if changes := lastReport(t, db).AgentChanges; len(changes) != 1 || changes[0].Section != "sec-1" {
		t.Fatalf("changes: %+v", changes)
	}
}

// B-103: only the step that read your direct message answers it; the chain's next
// one, on the app's «write the next section», is on its own.
func TestB103_ADirectMessagesStepIsNotOnItsOwnButTheNextOneIs(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Escribe la muestra."); err != nil {
		t.Fatal(err)
	}
	changes := lastReport(t, db).AgentChanges
	if len(changes) != 1 || changes[0].Step != 2 {
		t.Fatalf("changes: %+v, want only the second step", changes)
	}
}

// B-103: a message queued during a running step did not enter that step: the step
// stays on its own, and the next one, which read it, does not.
func TestB103_AMessageQueuedDuringAStepDoesNotExcuseThatStep(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	hold := &holdProvider{kind: provider.TurnStep, at: 2}
	runner, cap, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if turn, err := runner.StartTurn("p1", "Usa la serie de 2024."); err != nil || turn != "" {
		t.Fatalf("queue: %q %v", turn, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) < 3 {
		t.Fatalf("steps: %d", len(cap.sent))
	}
	changes := lastReport(t, db).AgentChanges
	if len(changes) != 1 || changes[0].Step != 2 {
		t.Fatalf("changes: %+v, want only the step that was running", changes)
	}
}

// B-103: limited to the stretch since the previous report.
func TestB103_AnEarlierStretchIsNotRepeated(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	if applied := applyRecorded(t, db, json.RawMessage(pendingFigureStep)); !applied.Applied {
		t.Fatal(applied.Errors)
	}
	runner, _, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"), textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue."); err != nil {
		t.Fatal(err)
	}
	first := lastReport(t, db)
	if len(first.Decided) != 1 || len(first.AgentChanges) != 1 {
		t.Fatalf("first report: %+v %+v", first.Decided, first.AgentChanges)
	}
	if applied := applyRecorded(t, db, json.RawMessage(pendingFigureStep)); !applied.Applied {
		t.Fatal(applied.Errors)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-3", "Sigue."); err != nil {
		t.Fatal(err)
	}
	second := lastReport(t, db)
	if len(second.Decided) != 0 || len(second.AgentChanges) != 1 || second.AgentChanges[0].Step != 1 {
		t.Fatalf("second report: %+v %+v", second.Decided, second.AgentChanges)
	}
}

// A report stored before B-103 still reads, with the two lists empty.
func TestB103_AReportStoredWithoutTheNewFieldsStillReads(t *testing.T) {
	db := cleanCardProject(t)
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent",
			Data: map[string]any{"stop_reason": "no_work", "section": "sec-1", "answer": "Paré.", "done": []string{"Algo."}}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	report, ok, err := cycle.LatestStopReport(context.Background(), db, "p1")
	if err != nil || !ok || report.Answer != "Paré." || len(report.Decided) != 0 || len(report.AgentChanges) != 0 {
		t.Fatalf("report: %+v %v %v", report, ok, err)
	}
}

// looseFigureProject has sec-1 with the loose figure «7», an agent pending, and
// the card answered, so the figure is what is left.
func looseFigureProject(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	if applied := applyRecorded(t, db, stepFile(t, "figure-datum-loose")); !applied.Applied {
		t.Fatal(applied.Errors)
	}
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara igual."); err != nil {
		t.Fatal(err)
	}
	return db
}

func sectionHasPending(t *testing.T, db *store.DB, kind string) bool {
	t.Helper()
	_, pendings := status(t, db, "sec-1")
	for _, p := range pendings {
		if p.Kind == kind {
			return true
		}
	}
	return false
}

// B-103 (c), Listo cuando: «Pedir corrección» launches ONE step, whose task names
// the pending and the section by label, and that step closes the pending.
func TestB103_ACorrectionRunsOneStepAndThePendingCloses(t *testing.T) {
	db := looseFigureProject(t)
	closeStretch(t, db)
	step := editSection(t, db, "Quito los 7 casos sueltos.", " y deja 7 casos.", ".")
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
	turn, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || turn == "" {
		t.Fatalf("RequestCorrection: %q %v", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("steps: %d, want one", len(cap.sent))
	}
	task := lastUserText(t, cap.sent[0])
	for _, want := range []string{"sec-1", "figure_without_decision", "7"} {
		if !strings.Contains(task, want) {
			t.Fatalf("the task misses %q: %s", want, task)
		}
	}
	if strings.Contains(task, correctionText) {
		t.Fatalf("the page's message went in the task: %s", task)
	}
	if sectionHasPending(t, db, graph.PendingFigureWithoutDecision) {
		t.Fatal("the pending did not close")
	}
	report := lastReport(t, db)
	if report.StopReason != cycle.StopCorrectionApplied {
		t.Fatalf("reason: %s", report.StopReason)
	}
	if len(report.AgentChanges) != 0 {
		t.Fatalf("the step of «Pedir corrección» is listed as on its own: %+v", report.AgentChanges)
	}
}

// B-103 (c), Listo cuando: over a cap (B-89) no call goes out.
func TestB103_ACorrectionRespectsTheCap(t *testing.T) {
	db := looseFigureProject(t)
	spend(t, db, "p1", 2.50)
	runner, cap, pub := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	turn, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || turn == "" {
		t.Fatalf("RequestCorrection: %q %v", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	wantCapStop(t, db, cap, pub, "project_cap", "")
}

// B-103 (c), Listo cuando: with a turn running it stays queued as a message of
// yours, in literal, visible, and the running chain's next step reads it.
func TestB103_ACorrectionWithATurnRunningStaysQueued(t *testing.T) {
	db := looseFigureProject(t)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	runner, cap, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	turn, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || turn != "" {
		t.Fatalf("RequestCorrection: %q %v, want queued", turn, err)
	}
	queued, err := runner.QueuedMessages(context.Background(), "p1")
	if err != nil || len(queued) != 1 || queued[0].Text != correctionText {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	if err := runner.WithdrawQueuedMessage(context.Background(), "p1", queued[0].N); err != nil {
		t.Fatalf("it is yours to remove: %v", err)
	}
	if _, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) < 2 || lastUserText(t, cap.sent[1]) != correctionText {
		t.Fatalf("the next step's message: %v", cap.sent)
	}
}

// B-103 (c), during the mode: its chain reads the queue.
func TestB103_ACorrectionDuringTheModeQueues(t *testing.T) {
	db := looseFigureProject(t)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	runner, cap, _ := modeRunnerOn(t, db, hold, textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.SetMode(context.Background(), "p1", graph.ModeRunToCompletion, ""); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	turn, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || turn != "" {
		t.Fatalf("RequestCorrection: %q %v, want queued", turn, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) < 2 || lastUserText(t, cap.sent[1]) != correctionText {
		t.Fatalf("the next step's message: %d calls", len(cap.sent))
	}
}

// B-103 (c): Go checks the pending is really open on that section; what is yours
// is not the agent's to correct. No call goes out.
func TestB103_ACorrectionOfAClosedPendingIsRefused(t *testing.T) {
	db := looseFigureProject(t)
	runner, cap, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.RequestCorrection(ctx, "p1", "sec-1", graph.PendingFigureWithoutDecision, "99", correctionText); !errors.Is(err, cycle.ErrPendingNotOpen) {
		t.Fatalf("another figure: %v", err)
	}
	if _, err := runner.RequestCorrection(ctx, "p1", "sec-1", graph.PendingOpenCard, "card-1", correctionText); !errors.Is(err, cycle.ErrPendingYours) {
		t.Fatalf("a card: %v", err)
	}
	if _, err := runner.RequestCorrection(ctx, "p1", "sec-99", graph.PendingFigureWithoutDecision, "7", correctionText); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("no such section: %v", err)
	}
	if _, err := runner.RequestCorrection(ctx, "p1", "sec-1", cycle.CorrectionOpenChange, "chg-9", correctionText); !errors.Is(err, cycle.ErrPendingNotOpen) {
		t.Fatalf("no such change: %v", err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 0 {
		t.Fatalf("calls: %d", len(cap.sent))
	}
}

// B-103 (c): an open change gets its button too: the task names it for handled_changes.
func TestB103_ACorrectionOfAnOpenChangeNamesIt(t *testing.T) {
	db := cleanCardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	sec, _ := object(t, db, "sec-1")
	changes := graph.OpenChanges(sec)
	if len(changes) == 0 {
		t.Fatal("setup: no open change")
	}
	runner, cap, _ := modeRunnerOn(t, db, nil, textFixture(t, closesChange(t)), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", cycle.CorrectionOpenChange, changes[0].Label, "Corrige el cambio."); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	task := lastUserText(t, cap.sent[0])
	if !strings.Contains(task, changes[0].Label) || !strings.Contains(task, "handled_changes") {
		t.Fatalf("the task: %s", task)
	}
	if sec, _ := object(t, db, "sec-1"); len(graph.OpenChanges(sec)) != 0 {
		t.Fatal("the change is still open")
	}
}

// B-103: a judge's correction batch is on its own, even in a step your message opened.
func TestB103_AJudgesCorrectionBatchIsOnItsOwn(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flawOne, "figures_and_calculations")))
	seedJudged(t, db)
	ctx := context.Background()
	if _, err := runner.RunJudgeVerify(ctx, "p1", "sec-1", cycle.JudgeReasonRisk, nil); err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	closeStretch(t, db)
	cap.inner = recordingFake(db, provider.NewFake(batchStep(t, db)))
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	changes := lastReport(t, db).AgentChanges
	if len(changes) != 1 || changes[0].Section != "sec-1" || changes[0].Summary != "Verificar: corrijo." ||
		changes[0].Changes[0].In != "Revisamos además los gruesos." {
		t.Fatalf("changes: %+v", changes)
	}
}

// decidedWith reads the stretch's decided entries with one statement.
func decidedWith(t *testing.T, db *store.DB, statement string) []graph.ReportDecided {
	t.Helper()
	decided, err := graph.ReportDecisions(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var out []graph.ReportDecided
	for _, d := range decided {
		if d.Statement == statement {
			out = append(out, d)
		}
	}
	return out
}

func statementOf(t *testing.T, db *store.DB, label string) string {
	t.Helper()
	_, body := object(t, db, label)
	statement, _ := body["statement"].(string)
	return statement
}

// B-103 (a), verifier's B1: keeping after an objection writes to:"human"; the
// decision is listed once, with the reason you gave.
func TestB103_ADecisionKeptAfterAnObjectionIsListedWithYourReason(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	objected, err := gate.ObjectDecision(ctx, "p1", "dec-2", "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	keep := 0
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", objected.OpenQuestion, &keep, catchUpReasonTwo); err != nil {
		t.Fatal(err)
	}
	got := decidedWith(t, db, statementOf(t, db, "dec-2"))
	if len(got) != 1 || got[0].Reason != catchUpReasonTwo || got[0].Who != "human" {
		t.Fatalf("decided: %+v", got)
	}
}

// B1: an ordinary «por aclarar» kept writes no event on the decision; its own
// answering event says it.
func TestB103_ADecisionKeptOnAnOrdinaryQuestionIsListedWithYourReason(t *testing.T) {
	db := cleanCardProject(t)
	ctx := context.Background()
	dec, _ := object(t, db, "dec-2")
	var label string
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		q, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{"question": "¿Sigue valiendo?",
			"options": []any{map[string]any{"option": "Mantener", "keep": true}, map[string]any{"option": "Cambiarla"}},
			"keeps":   map[string]any{"kind": "decision", "id": dec.ID}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		label = q.Label
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	closeStretch(t, db)
	keep := 0
	if _, err := cycle.NewGate(db).AnswerOpenQuestion(ctx, "p1", label, &keep, catchUpReasonTwo); err != nil {
		t.Fatal(err)
	}
	got := decidedWith(t, db, statementOf(t, db, "dec-2"))
	if len(got) != 1 || got[0].Reason != catchUpReasonTwo || got[0].Who != "human" {
		t.Fatalf("decided: %+v", got)
	}
}

// B1: a decision born in the stretch and kept in it lists the new reason, once.
func TestB103_ADecisionBornAndKeptInTheStretchListsTheNewReason(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	ctx := context.Background()
	gate := cycle.NewGate(db)
	answer, err := gate.AnswerCard(ctx, "p1", "card-1", cycle.OptionOther, catchUpReasonOne)
	if err != nil {
		t.Fatal(err)
	}
	objected, err := gate.ObjectDecision(ctx, "p1", answer.NewDecision, "Porque quiero revisar esta decisión.")
	if err != nil {
		t.Fatal(err)
	}
	keep := 0
	if _, err := gate.AnswerOpenQuestion(ctx, "p1", objected.OpenQuestion, &keep, catchUpReasonTwo); err != nil {
		t.Fatal(err)
	}
	got := decidedWith(t, db, statementOf(t, db, answer.NewDecision))
	if len(got) != 1 || got[0].Reason != catchUpReasonTwo {
		t.Fatalf("decided: %+v", got)
	}
}

func humanMessages(t *testing.T, db *store.DB) int {
	t.Helper()
	return len(eventsOfType(t, db, store.EventHumanMessage))
}

// B2: a second request for the pending the running step is correcting answers with
// that turn, writes nothing and runs no other step.
func TestB103_ASecondRequestForTheRunningCorrectionQueuesNothing(t *testing.T) {
	db := looseFigureProject(t)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	runner, cap, _ := modeRunnerOn(t, db, hold, noWork, noWork, noWork)
	ctx := context.Background()
	turn, err := runner.RequestCorrection(ctx, "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || turn == "" {
		t.Fatalf("first: %q %v", turn, err)
	}
	<-hold.entered
	before := humanMessages(t, db)
	again, err := runner.RequestCorrection(ctx, "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText)
	if err != nil || again != turn {
		t.Fatalf("second: %q %v, want the running turn %q", again, err, turn)
	}
	if humanMessages(t, db) != before {
		t.Fatal("the second request wrote a message")
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 1 {
		t.Fatalf("steps: %d, want one", len(cap.sent))
	}
}

// B2: the same correction asked twice with another turn running queues one message.
func TestB103_ARepeatedCorrectionIsNotQueuedTwice(t *testing.T) {
	db := looseFigureProject(t)
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	runner, _, _ := modeRunnerOn(t, db, hold, noWork, noWork, noWork)
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	for range 2 {
		if turn, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText); err != nil || turn != "" {
			t.Fatalf("queue: %q %v", turn, err)
		}
	}
	queued, err := runner.QueuedMessages(context.Background(), "p1")
	if err != nil || len(queued) != 1 {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
}

// B3: a step with no work leaves the pending open, and the stop says so.
func TestB103_ACorrectionThatDidNothingSaysItIsStillOpen(t *testing.T) {
	db := looseFigureProject(t)
	runner, _, pub := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	if got := pub.step(t)["stop_reason"]; got != string(cycle.StopCorrectionStillOpen) {
		t.Fatalf("stop reason: %v", got)
	}
	if !sectionHasPending(t, db, graph.PendingFigureWithoutDecision) {
		t.Fatal("setup: the pending closed")
	}
}

// B3 and A1: a step that wrote elsewhere stops saying it is still open; the section
// the task names is excused, the other one goes in agent_changes.
func TestB103_ACorrectionThatWroteElsewhereSaysItIsStillOpenAndListsIt(t *testing.T) {
	db := looseFigureProject(t)
	closeStretch(t, db)
	sec, _ := object(t, db, "sec-1")
	step := strings.Replace(string(stepFile(t, "new-section-and-edit")), `"base_version": 1`, `"base_version": `+jsonInt(sec.CurrentVersion), 1)
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.RequestCorrection(context.Background(), "p1", "sec-1", graph.PendingFigureWithoutDecision, "7", correctionText); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	report := lastReport(t, db)
	if report.StopReason != cycle.StopCorrectionStillOpen {
		t.Fatalf("reason: %s", report.StopReason)
	}
	if len(report.AgentChanges) != 1 || report.AgentChanges[0].Section == "sec-1" {
		t.Fatalf("changes: %+v, want only the other section", report.AgentChanges)
	}
}

// A1: so does the one step that applies your decision.
func TestB103_TheApplyStepListsTheOtherSectionItTouched(t *testing.T) {
	db := cleanCardProject(t)
	closeStretch(t, db)
	sec, _ := object(t, db, "sec-1")
	step := strings.Replace(string(stepFile(t, "new-section-and-edit")), `"base_version": 1`, `"base_version": `+jsonInt(sec.CurrentVersion+1), 1)
	step = strings.Replace(step, `"note": "Paso de prueba."`, `"note": "Paso de prueba.", "handled_changes": [{"change_id": "chg-1", "result": "updated"}]`, 1)
	runner, _, _ := modeRunnerOn(t, db, nil, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, catchUpReasonOne); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	report := lastReport(t, db)
	if report.StopReason != cycle.StopDecisionApplied {
		t.Fatalf("reason: %s", report.StopReason)
	}
	if len(report.AgentChanges) != 1 || report.AgentChanges[0].Section == "sec-1" {
		t.Fatalf("changes: %+v", report.AgentChanges)
	}
}

// A3, rule 7: an internal id in the path never reaches the model.
func TestB103_TheTaskNamesTheSectionByLabelEvenFromAnInternalID(t *testing.T) {
	db := looseFigureProject(t)
	sec, _ := object(t, db, "sec-1")
	runner, cap, _ := modeRunnerOn(t, db, nil, loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.RequestCorrection(context.Background(), "p1", sec.ID, graph.PendingFigureWithoutDecision, "7", correctionText); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	task := lastUserText(t, cap.sent[0])
	if strings.Contains(task, sec.ID) || !strings.Contains(task, "sec-1") {
		t.Fatalf("the task: %s", task)
	}
}
