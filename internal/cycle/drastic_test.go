package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// editStep is a step that makes these edits on sec-1, read at its current version.
func editStep(t *testing.T, db *store.DB, edits ...map[string]any) provider.Fixture {
	t.Helper()
	sec, _ := object(t, db, "sec-1")
	for _, e := range edits {
		e["section_id"], e["base_version"], e["claim_origin"] = "sec-1", sec.CurrentVersion, "material"
		if e["operation"] == nil {
			e["operation"] = "replace"
		}
	}
	raw, err := json.Marshal(map[string]any{"stretch": "build", "triage_line": "Corrijo.", "closing_risk": "none", "note": "Corrijo.", "new_text": edits})
	if err != nil {
		t.Fatal(err)
	}
	return textFixture(t, string(raw))
}

func proposalsFor(t *testing.T, db *store.DB, section string) int {
	t.Helper()
	n := 0
	for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
		var d map[string]any
		_ = json.Unmarshal(ev.Data, &d)
		if ev.Actor == "agent" && d["section"] == section && d["to"] == "open" {
			n++
		}
	}
	return n
}

func sectionOneText(t *testing.T, db *store.DB) string {
	t.Helper()
	_, b := object(t, db, "sec-1")
	return b["text"].(string)
}

// inBatch leaves a judge's flaw waiting, so the next step is its correction batch.
func inBatch(t *testing.T, flaw string) (*store.DB, *cycle.Runner, *captureProvider, string) {
	t.Helper()
	db, runner, cap, _ := newCaptureRunner(t, verifyFlaws(t, oneFlaw("sec-1", flaw, "figures_and_calculations")))
	seedJudged(t, db)
	label, err := runner.RunJudgeVerify(context.Background(), "p1", "sec-1", cycle.JudgeReasonRisk, nil)
	if err != nil {
		t.Fatalf("RunJudgeVerify: %v", err)
	}
	return db, runner, cap, label
}

// B-99 (a): a batch that takes out more than half of the section waits as a proposal.
func TestB99_ABatchThatRemovesMoreThanHalfStaysAsAProposal(t *testing.T) {
	db, runner, cap, _ := inBatch(t, flawOne)
	before := sectionOneText(t, db)
	cap.inner = recordingFake(db, provider.NewFake(editStep(t, db,
		map[string]any{"anchor": flawOne, "text": ""}, map[string]any{"anchor": flawTwo, "text": ""}),
		loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := sectionOneText(t, db); got != before {
		t.Fatalf("the text changed: %q", got)
	}
	if n := proposalsFor(t, db, "sec-1"); n != 1 {
		t.Fatalf("proposals: %d, want 1", n)
	}
}

// B-99, Eber 3-nov, the case of ux-p1: an ordinary step too, not only a batch.
func TestB99_AnOrdinaryStepThatRemovesMoreThanHalfStaysAsAProposal(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	before := sectionOneText(t, db)
	cap.inner = recordingFake(db, provider.NewFake(editStep(t, db,
		map[string]any{"anchor": flawOne, "text": ""}, map[string]any{"anchor": flawTwo, "text": ""}),
		loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Quita lo que sobra."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := sectionOneText(t, db); got != before {
		t.Fatalf("the text changed: %q", got)
	}
	if n := proposalsFor(t, db, "sec-1"); n != 1 {
		t.Fatalf("proposals: %d, want 1", n)
	}
}

// B-99: an insertion in the middle of a sentence is rejected and the agent retries.
func TestB99_AnInsertionInTheMiddleOfASentenceIsRejectedAndRetried(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t)
	seedJudged(t, db)
	cap.inner = recordingFake(db, provider.NewFake(
		editStep(t, db, map[string]any{"anchor": "subieron 60 veces", "operation": "insert_after", "text": "Otra frase."}),
		editStep(t, db, map[string]any{"anchor": flawOne, "operation": "insert_after", "text": "Otra frase."}),
		loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Agrega."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	found := false
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		found = found || strings.Contains(string(ev.Data), "does not end a sentence")
	}
	if !found {
		t.Fatal("no rejection says the anchor does not end a sentence")
	}
	if got := sectionOneText(t, db); !strings.Contains(got, "Otra frase.") || strings.Contains(got, "subieron 60 veces.Otra") {
		t.Fatalf("after the retry: %q", got)
	}
}

// B-99: a small correction applies, and leaves its «Sale / Entra» with the flaw.
func TestB99_ASmallCorrectionAppliesAndLeavesItsChangeWithTheFlaw(t *testing.T) {
	db, runner, cap, label := inBatch(t, flawOne)
	const entering = "Los conteos subieron en 2023."
	cap.inner = recordingFake(db, provider.NewFake(editStep(t, db, map[string]any{"anchor": flawOne, "text": entering}),
		loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := sectionOneText(t, db); !strings.Contains(got, entering) {
		t.Fatalf("the text: %q", got)
	}
	if n := proposalsFor(t, db, "sec-1"); n != 0 {
		t.Fatalf("proposals: %d, want none", n)
	}
	view, err := runner.Round(context.Background(), "p1", label)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Changes) != 1 || view.Changes[0].Out != flawOne || view.Changes[0].In != entering ||
		view.Changes[0].N != 1 || view.Changes[0].Scenario == "" {
		t.Fatalf("the card's changes: %+v", view.Changes)
	}
}

// B-99, Eber 3-nov, the case-5 batch: rewriting the end of a sentence waits as a
// proposal in a batch, and applies in an ordinary step.
func TestB99_RewritingASentenceEndIsAProposalOnlyInABatch(t *testing.T) {
	const anchor = "en 2023."
	replacement := map[string]any{"anchor": anchor, "text": "cálculo previo no comparable."}
	db, runner, cap, _ := inBatch(t, flawOne)
	cap.inner = recordingFake(db, provider.NewFake(editStep(t, db, replacement), loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if n := proposalsFor(t, db, "sec-1"); n != 1 {
		t.Fatalf("in a batch, proposals: %d, want 1", n)
	}

	plain, runner2, cap2, _ := newCaptureRunner(t)
	seedJudged(t, plain)
	cap2.inner = recordingFake(plain, provider.NewFake(editStep(t, plain, map[string]any{"anchor": anchor, "text": "cálculo previo no comparable."}),
		loadAppFixture(t, "04-principal-no-work.json")))
	if err := runner2.RunTurn(context.Background(), "p1", "turn-1", "Corrige."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if n := proposalsFor(t, plain, "sec-1"); n != 0 {
		t.Fatalf("in an ordinary step, proposals: %d, want none", n)
	}
	if got := sectionOneText(t, plain); !strings.Contains(got, "cálculo previo") {
		t.Fatalf("the text: %q", got)
	}
}
