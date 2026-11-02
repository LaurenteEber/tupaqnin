package cycle_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-75 (b): fichas §2 names three entries for a stop; a chain that did
// no work since the last stop (answering a question is none) still
// stops, but asks for no report. A chain that did work still asks.
func TestP75_AChainWithoutWorkDoesNotAskForAReport(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t,
		loadAppFixture(t, "01-principal-valid.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn 1: %v", err)
	}
	if len(cap.reports) != 1 || len(stopReports(t, db)) != 1 {
		t.Fatalf("after a chain with work: %d report calls, %d stop reports; want 1 and 1",
			len(cap.reports), len(stopReports(t, db)))
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "¿Por qué esa regla?"); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want 2", len(cap.sent))
	}
	if len(cap.reports) != 1 || len(stopReports(t, db)) != 1 {
		t.Fatalf("after a chain without work: %d report calls, %d stop reports; want still 1 and 1",
			len(cap.reports), len(stopReports(t, db)))
	}
	if got := pub.stepAt(t, 1, 2)["stop_reason"]; got != "no_work" {
		t.Fatalf("published stop_reason: %v (the chain still stops)", got)
	}
}

// P-75 (b): a chain that stops without work in a project that never
// stopped asks for no report either.
func TestP75_AFirstChainWithoutWorkDoesNotAskForAReport(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Hola."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.reports) != 0 || len(stopReports(t, db)) != 0 {
		t.Fatalf("%d report calls, %d stop reports; want none", len(cap.reports), len(stopReports(t, db)))
	}
}

// P-75 (c): a report that fails twice blocks no signoff (estados §1 ties
// the agent's pendings to a section; the report writes in none) and the
// stop_report says it is incomplete (PrincipalFailed), with the app's
// part (B-41).
func TestP75_ATwiceFailedReportBlocksNoSignoffAndShowsIncomplete(t *testing.T) {
	bad := reportFixture(t, `{"answer": "Paré.", "done": [], "next": "Seguir.", "unseen": [{"decision": "dec-9", "gist": "x", "sections": ["sec-1"]}]}`)
	db, runner, cap, _ := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"), bad)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.reports) != 2 {
		t.Fatalf("report calls: %d, want 2", len(cap.reports))
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || !reports[0].PrincipalFailed || !reports[0].ProposeSignoff {
		t.Fatalf("stop reports: %+v", reports)
	}
	assertNoReportPending := func(when string) {
		t.Helper()
		_, pendings, err := graph.SectionStatus(context.Background(), db, "p1", "sec-1")
		if err != nil {
			t.Fatalf("SectionStatus: %v", err)
		}
		for _, p := range pendings {
			if p.Kind == graph.PendingMalformedOutput {
				t.Fatalf("%s: sec-1 has the pending %s", when, p.Kind)
			}
		}
		st, err := memory.New(db).WorkingState(context.Background(), "p1")
		if err != nil {
			t.Fatalf("WorkingState: %v", err)
		}
		if len(st.InFlight.AgentPending) != 0 {
			t.Fatalf("%s: agent pendings in the working state: %v", when, st.InFlight.AgentPending)
		}
		open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
		if err != nil || !open.Opened {
			t.Fatalf("%s: RequestSignoff: %+v %v (the failed report must block nothing)", when, open, err)
		}
		if err := cycle.NewGate(db).LeaveSignoff(context.Background(), "p1", open.Label); err != nil {
			t.Fatalf("LeaveSignoff: %v", err)
		}
	}
	assertNoReportPending("after the failed report")

	// A history written before P-75 keeps the report's output_retry with
	// result agent_pending; it blocks nothing either.
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventOutputRetry, Actor: "agent", AgentRole: "principal",
			Data: map[string]any{"turn_kind": "stop_report", "attempt": 2, "errors": []string{"provider_error"},
				"result": "agent_pending"}})
		return err
	}); err != nil {
		t.Fatalf("seed legacy retry: %v", err)
	}
	assertNoReportPending("with a legacy report retry")
}

// A well-formed report is not marked incomplete.
func TestP75_AWellFormedReportIsNotIncomplete(t *testing.T) {
	db, runner, _, _ := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].PrincipalFailed || reports[0].Answer == "" {
		t.Fatalf("stop reports: %+v", reports)
	}
}

// rejectedThenClean declares a close rule 7 rejects (since 4.1 a text on
// sec-1 while it is being signed is a proposal, B-73, so a close is what
// PendingRejections still carries) and a new section with text and no
// pendings, so the chain stops right after this step (section_ready).
const rejectedThenClean = `{"stretch": "build", "triage_line": "Construir.", "closing_risk": "none", "note": "n",
  "new_text": [{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "# Muestra\n\nRevisamos estudios de la cuenca."}],
  "handled_changes": [{"change_id": "chg-9", "result": "reviewed_unchanged", "reason": "No afecta a esta sección porque habla de otra cosa."}],
  "proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Muestra"}]}`

// rejectedMessage starts the message of the close rule 7 rejects.
const rejectedMessage = "chg-9 is not in the list of changes"

// B-14 (P-75): when the chain stops right after the rejected step, the
// rejection reaches the principal in its next turn, once.
func TestP75_TheRejectionReachesTheNextTurnWhenTheChainStoppedAfterIt(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("rejected", rejectedThenClean),
		loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, true)
	openSignoff(t, cycle.NewGate(db))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn 1: %v", err)
	}
	if len(cap.sent) != 1 || len(stopReports(t, db)) != 1 {
		t.Fatalf("turn 1: %d step calls, %d stop reports; want the chain to stop after the rejected step",
			len(cap.sent), len(stopReports(t, db)))
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	got := lastUserText(t, cap.sent[1])
	if !strings.HasPrefix(got, rejectedMessage) || !strings.HasSuffix(got, "Sigue") {
		t.Fatalf("next turn's message: %q", got)
	}
	// Turn 3: the notice was delivered once; it is not repeated.
	if err := runner.RunTurn(context.Background(), "p1", "turn-3", "Sigue"); err != nil {
		t.Fatalf("RunTurn 3: %v", err)
	}
	if got := lastUserText(t, cap.sent[2]); got != "Sigue" {
		t.Fatalf("third turn's message: %q, want only the person's", got)
	}
}

// B-14 (P-75): after a restart (a new instance with its startup), the
// pending rejection still reaches the principal's next turn, once.
func TestP75_TheRejectionReachesTheStartupAfterARestart(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, stepFixture("rejected", rejectedThenClean),
		loadAppFixture(t, "04-principal-no-work.json"))
	seedSignable(t, db, true)
	openSignoff(t, cycle.NewGate(db))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn 1: %v", err)
	}
	later := time.Now().Add(3 * time.Hour)
	runner.Now = func() time.Time { return later }
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want 2", len(cap.sent))
	}
	n := 0
	for _, item := range cap.sent[1] {
		n += strings.Count(string(item), rejectedMessage)
	}
	if n != 1 {
		t.Fatalf("the startup carries the rejection %d times, want 1", n)
	}
}

// B-14: a rejection followed by another chained step reaches that step
// once, and not the next turn again.
func TestP75_TheRejectionReachesTheNextChainedStepOnlyOnce(t *testing.T) {
	onSigning := stepFixture("on-signing", `{"stretch": "build", "triage_line": "Construir.", "closing_risk": "none", "note": "n",
	  "new_text": [{"section_id": "new-1", "base_version": 0, "anchor": "", "claim_origin": "material", "operation": "insert_after", "text": "Revisamos 12 estudios."}],
	  "handled_changes": [{"change_id": "chg-9", "result": "reviewed_unchanged", "reason": "No afecta a esta sección porque habla de otra cosa."}],
	  "proposed_roles": [{"section_id": "new-1", "role": "other", "title": "Muestra"}]}`)
	noWork := loadAppFixture(t, "04-principal-no-work.json")
	db, runner, cap, _ := newCaptureRunner(t, onSigning, noWork, noWork)
	seedSignable(t, db, true)
	openSignoff(t, cycle.NewGate(db))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Mejora la sección."); err != nil {
		t.Fatalf("RunTurn 1: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want 2", len(cap.sent))
	}
	if got := lastUserText(t, cap.sent[1]); strings.Count(got, rejectedMessage) != 1 {
		t.Fatalf("second step message: %q", got)
	}
	if err := runner.RunTurn(context.Background(), "p1", "turn-2", "Sigue"); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	if got := lastUserText(t, cap.sent[2]); got != "Sigue" {
		t.Fatalf("next turn's message: %q, want only the person's", got)
	}
}
