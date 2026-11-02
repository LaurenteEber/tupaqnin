package cycle_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// pendingFigureStep writes a new section whose figure has no decision,
// so the section keeps a pending of the agent: no stop reason of the
// stop holds after it (backlog-r2 2.5).
const pendingFigureStep = `{"stretch": "build", "triage_line": "Construir: escribo la muestra.", "closing_risk": "none",` +
	`"new_text": [{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material",` +
	`"operation": "insert_after", "text": "# Muestra\n\nRevisamos 12 estudios de la cuenca."}],` +
	`"proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Muestra"}],` +
	`"note": "Escribí la muestra."}`

// reportFixture is a stop report recording of the principal.
func reportFixture(t *testing.T, text string) provider.Fixture {
	t.Helper()
	fx := textFixture(t, text)
	fx.TurnKind = provider.TurnStopReport
	return fx
}

func stopReports(t *testing.T, db *store.DB) []cycle.StopReport {
	t.Helper()
	var out []cycle.StopReport
	for _, ev := range eventsOfType(t, db, store.EventStopReport) {
		var r cycle.StopReport
		if err := json.Unmarshal(ev.Data, &r); err != nil {
			t.Fatalf("stop_report data: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// B-39: with 5 decisions of the agent you have not seen, the chain stops
// and asks the principal for the report, with the app's list of them.
func TestB39_FiveUnseenDecisionsStopTheChainAndAskForTheReport(t *testing.T) {
	var decisions []map[string]any
	for i := 2; i <= 6; i++ {
		decisions = append(decisions, map[string]any{"id": "new-" + string(rune('0'+i)),
			"statement": "Regla " + string(rune('0'+i)) + " del alcance.", "instead_of": "Otra regla.",
			"would_choose_if": "Cambiara el pedido.", "assumption": false, "used_in_sections": []string{"new-1"}})
	}
	step, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Construir: fijo el alcance.",
		"closing_risk": "none", "note": "Registré cinco reglas.",
		"new_text": []map[string]any{{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material",
			"operation": "insert_after", "text": "# Alcance\n\nEl alcance sigue cinco reglas."}},
		"proposed_roles": []map[string]any{{"section_id": "new-1", "role": "other", "title": "Alcance"}},
		"new_decisions":  decisions})
	if err != nil {
		t.Fatal(err)
	}
	db, runner, cap, pub := newCaptureRunner(t, textFixture(t, string(step)))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe el alcance."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("step calls: %d, want 1 (the chain stops after the fifth unseen decision)", len(cap.sent))
	}
	if len(cap.reports) != 1 || len(cap.reports[0].ReportDecisions) != 5 {
		t.Fatalf("report calls: %+v, want one with the 5 unseen decisions", cap.reports)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopUnseenDecisions || len(reports[0].Decisions) != 5 ||
		len(reports[0].Unseen) != 5 || reports[0].Answer == "" {
		t.Fatalf("stop reports: %+v", reports)
	}
	if got := pub.step(t)["stop_reason"]; got != "unseen_decisions" {
		t.Fatalf("published stop_reason: %v", got)
	}
}

// Fichas §2: a section the chain wrote, with text and no pendings, stops
// the chain; the report evaluates it and the app proposes signing it.
func TestB37_ASectionWithoutPendingsStopsTheChain(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 1 || len(cap.reports) != 1 {
		t.Fatalf("step calls %d, report calls %d: want 1 and 1", len(cap.sent), len(cap.reports))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopSectionReady || reports[0].Section != "sec-1" ||
		!reports[0].ProposeSignoff {
		t.Fatalf("stop reports: %+v", reports)
	}
	if got := pub.step(t)["stop_reason"]; got != "section_ready" {
		t.Fatalf("published stop_reason: %v", got)
	}
}

// Fichas §2: when only cards and «por aclarar» are left, the chain stops
// («solo queda lo tuyo»).
func TestB37_OnlyCardsAndOpenQuestionsLeftStopTheChain(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, loadAppFixture(t, "01b-principal-critical-decision.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la regla."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	open, err := db.ListObjects(context.Background(), "p1", store.ObjectCard)
	if err != nil || len(open) != 1 || open[0].State != "open" {
		t.Fatalf("cards: %+v %v, want one open", open, err)
	}
	if len(cap.sent) != 1 {
		t.Fatalf("step calls: %d, want 1", len(cap.sent))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopHumanOnly || reports[0].ProposeSignoff ||
		len(reports[0].Open) != 1 || reports[0].Open[0].Kind != "card" {
		t.Fatalf("stop reports: %+v", reports)
	}
}

// B-37: the chain stops at 12 steps with «tope de pasos», in step_ended
// and in the report.
func TestB37_ChainStopsAtTwelveStepsWithStepLimit(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t, textFixture(t, pendingFigureStep))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if cycle.MaxChainSteps != 12 || len(cap.sent) != 12 {
		t.Fatalf("MaxChainSteps %d, step calls %d: want 12", cycle.MaxChainSteps, len(cap.sent))
	}
	steps := stepEndedData(t, db)
	if len(steps) != 12 || steps[11]["stop_reason"] != "step_limit" || steps[10]["stop_reason"] != nil {
		t.Fatalf("stop_reason of the last two steps: %v, %v", steps[10]["stop_reason"], steps[11]["stop_reason"])
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopStepLimit {
		t.Fatalf("stop reports: %+v", reports)
	}
	if got := pub.step(t)["stop_reason"]; got != "step_limit" {
		t.Fatalf("published stop_reason: %v", got)
	}
}

// seedSectionWithOpenChange creates sec-1 with text and an open change.
func seedSectionWithOpenChange(t *testing.T, db *store.DB) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "stale", map[string]any{
			"title": "Alcance", "text": "El alcance cubre la cuenca.",
			"open_changes": []map[string]any{{"label": "chg-1", "kind": "decision", "about": "dec-9"}},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed section: %v", err)
	}
}

// Fichas §2 and backlog-r2 2.5: the report proposes signing only when
// the section has no pendings and no open changes, computed by Go; a
// principal that says it is ready does not change that.
func TestB37_ReportDoesNotProposeSigningWithAnOpenChangeEvenIfTheModelAsks(t *testing.T) {
	write := `{"stretch": "build", "triage_line": "Construir: amplío el alcance.", "closing_risk": "none",` +
		`"new_text": [{"section_id": "sec-1", "base_version": 1, "anchor": "El alcance cubre la cuenca.",` +
		`"claim_origin": "material", "operation": "insert_after", "text": "También cubre sus afluentes."}],` +
		`"note": "Amplié el alcance."}`
	asks := `{"answer": "El alcance está listo para firmar: fírmalo.", "done": ["Amplié el alcance."],` +
		`"next": "Firmar el alcance.", "unseen": []}`
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, write), loadAppFixture(t, "04-principal-no-work.json"),
		reportFixture(t, asks))
	seedSectionWithOpenChange(t, db)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Amplía el alcance."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want 2 (the open change keeps the chain going until no work)", len(cap.sent))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].StopReason != cycle.StopNoWork || reports[0].Section != "sec-1" ||
		reports[0].ProposeSignoff || reports[0].Answer != "El alcance está listo para firmar: fírmalo." {
		t.Fatalf("stop reports: %+v", reports)
	}
}

// Fichas §7: a report outside its fixed shape is retried once; a second
// failure keeps the report with the app's part only.
func TestB41_AReportOutsideItsShapeIsRetriedOnceThenKeepsTheAppsPart(t *testing.T) {
	bad := reportFixture(t, `{"answer": "Paré.", "done": [], "next": "Seguir.", "unseen": [{"decision": "dec-9", "gist": "x", "sections": ["sec-1"]}]}`)
	db, runner, cap, _ := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"), bad)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.reports) != 2 {
		t.Fatalf("report calls: %d, want 2 (one retry)", len(cap.reports))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || !reports[0].PrincipalFailed || reports[0].Answer != "" || reports[0].Section != "sec-1" {
		t.Fatalf("stop reports: %+v", reports)
	}
	retries := eventsOfType(t, db, store.EventOutputRetry)
	if len(retries) != 2 || eventData(t, retries[1])["turn_kind"] != "stop_report" {
		t.Fatalf("output_retry: %d", len(retries))
	}
}

// B-40: opening the report records stop_report_seen and decision_seen
// (report) for each decision it shows, once.
func TestB40_OpeningTheReportRecordsDecisionSeenForEachDecisionItShows(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, loadAppFixture(t, "01b-principal-critical-decision.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la regla."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	report, ok, err := cycle.LatestStopReport(context.Background(), db, "p1")
	if err != nil || !ok || report.Seen || len(report.Decisions) != 1 {
		t.Fatalf("report before: %+v %v %v", report, ok, err)
	}
	gate := cycle.NewGate(db)
	for i := 0; i < 2; i++ {
		if err := gate.MarkStopReportSeen(context.Background(), "p1"); err != nil {
			t.Fatalf("MarkStopReportSeen: %v", err)
		}
	}
	if n := len(eventsOfType(t, db, store.EventStopReportSeen)); n != 1 {
		t.Fatalf("stop_report_seen: %d, want 1", n)
	}
	seen := eventsOfType(t, db, store.EventDecisionSeen)
	if len(seen) != 1 || eventData(t, seen[0])["seen_via"] != "report" || eventData(t, seen[0])["label"] != report.Decisions[0].Label {
		t.Fatalf("decision_seen: %d", len(seen))
	}
	if report, _, _ = cycle.LatestStopReport(context.Background(), db, "p1"); !report.Seen {
		t.Fatal("the report is still unseen after opening it")
	}
}

// The embedded report schema is the design's, byte for byte.
func TestB41_EmbeddedStopReportSchemaEqualsTheDesign(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "stop_report_output.schema.json"))
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-informe.schema.json"))
	if err != nil {
		t.Fatalf("read design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from docs/diseno/bloque-3/salida-informe.schema.json")
	}
}

// P-111, B-41: the report does not call verified a citation with an open
// source_not_supporting, whatever its cause, and it lists the agent's open
// pendings with their passage.
func TestB41_TheReportReadsTheCheckerAndListsTheAgentsPendings(t *testing.T) {
	db := seedCheckerProject(t)
	runCheckedTurn(t, db, nil, textFixture(t, string(stepFile(t, "citations"))), checkerFixture(t, "source-not-supporting"))
	reports := stopReports(t, db)
	if len(reports) != 1 {
		t.Fatalf("stop reports: %d", len(reports))
	}
	report := reports[0]
	for _, c := range report.Verified {
		if c.Claim == "Los estudios usan filtros." {
			t.Fatalf("verified lists a citation the checker marked: %+v", report.Verified)
		}
	}
	marked := false
	for _, c := range report.NotVerified {
		marked = marked || c.Claim == "Los estudios usan filtros."
	}
	if !marked {
		t.Fatalf("not verified: %+v", report.NotVerified)
	}
	kinds := map[string]string{}
	for _, p := range report.AgentPendings {
		if p.Section != "sec-1" {
			t.Fatalf("agent pending of another section: %+v", p)
		}
		kinds[p.Kind+":"+p.About] = p.Passage
	}
	if kinds[graph.PendingSourceNotSupporting+":cit-1"] != "Los estudios usan filtros." ||
		len(report.AgentPendings) != 2 {
		t.Fatalf("agent pendings: %+v", report.AgentPendings)
	}
}

// P-111: yours are not listed as the agent's: an open card stays in «open».
func TestB41_AReportWithoutAgentPendingsListsNone(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, textFixture(t, riskStep("none")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || len(reports[0].AgentPendings) != 0 {
		t.Fatalf("reports: %+v", reports)
	}
}
