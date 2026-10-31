package cycle_test

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// secondDefenseStep is a second step on sec-1 that replaces the passage
// the first step wrote with the given text.
func secondDefenseStep(text string) string {
	return `{"stretch":"build","triage_line":"Construir: repaso la regla.","closing_risk":"none","note":"Repasé la regla.",` +
		`"used_decisions":["dec-1"],` +
		`"new_text":[{"section_id":"sec-1","base_version":2,"anchor":"Así la comparación es justa y repetible.","claim_origin":"material",` +
		`"operation":"replace","text":"` + text + `"}]}`
}

// runTwoDefenseStops runs the defense's first turn and a second one
// whose step writes text, each stopping to propose signing sec-1.
func runTwoDefenseStops(t *testing.T, text string, fixtures ...provider.Fixture) (*store.DB, *captureProvider) {
	t.Helper()
	all := append([]provider.Fixture{textFixture(t, defenseStep), judgeFixture(t, "conclusion-section"),
		textFixture(t, secondDefenseStep(text))}, fixtures...)
	db, runner, cap, _ := newCaptureRunner(t, all...)
	seedDefense(t, db, "", "conclusion")
	ctx := context.Background()
	if err := runner.RunTurn(ctx, "p1", "turn-1", "Cierra la regla."); err != nil {
		t.Fatalf("RunTurn 1: %v", err)
	}
	if err := runner.RunTurn(ctx, "p1", "turn-2", "Repasa la regla."); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	reports := stopReports(t, db)
	if len(reports) != 2 || !reports[1].ProposeSignoff || reports[1].Section != "sec-1" {
		t.Fatalf("stop reports: %+v, want two proposing to sign sec-1", reports)
	}
	return db, cap
}

// P-76 (fichas §2 «one round»): a second stop that proposes signing a
// section whose text keeps the fingerprint of the last preparation
// (only its formatting changed) makes no judge call and adds no
// audience question; the open ones stay.
func TestP76_SameFingerprintSinceLastPreparationMakesNoJudgeCall(t *testing.T) {
	db, cap := runTwoDefenseStops(t, "Así la comparación es   justa y repetible.")
	if len(cap.judges) != 1 {
		t.Fatalf("judge calls: %d, want 1 (the text did not change since the last preparation)", len(cap.judges))
	}
	want := recordedDefense(t, judgeFixture(t, "conclusion-section"))
	open, err := graph.OpenAudienceQuestions(context.Background(), db, "p1", "sec-1")
	if err != nil {
		t.Fatalf("OpenAudienceQuestions: %v", err)
	}
	if len(open) != len(want.AudienceQuestions)+len(want.NotCovered) {
		t.Fatalf("open audience questions: %d, want the first preparation's %d",
			len(open), len(want.AudienceQuestions)+len(want.NotCovered))
	}
}

// P-76: the previous behaviour still holds when the text changed: the
// next stop that proposes signing prepares the defense again.
func TestP76_AChangedTextGetsANewPreparation(t *testing.T) {
	_, cap := runTwoDefenseStops(t, "Así la comparación es justa y reproducible.", judgeFixture(t, "conclusion-section"))
	if len(cap.judges) != 2 {
		t.Fatalf("judge calls: %d, want 2 (the text changed)", len(cap.judges))
	}
}
