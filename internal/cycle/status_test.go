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

// cleanCardProject: like cardProject, without the loose number, so the
// open card is sec-1's only pending.
func cleanCardProject(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	out := strings.Replace(string(stepFile(t, "figure-datum-loose")), " y deja 7 casos.", ".", 1)
	if applied := applyRecorded(t, db, json.RawMessage(out)); !applied.Applied {
		t.Fatalf("setup: %v", applied.Errors)
	}
	return db
}

func status(t *testing.T, db *store.DB, label string) (graph.Status, []graph.Pending) {
	t.Helper()
	st, pendings, err := graph.SectionStatus(context.Background(), db, "p1", label)
	if err != nil {
		t.Fatalf("SectionStatus: %v", err)
	}
	return st, pendings
}

func TestE1_OpenCardLeavesTheSectionInDraftAndAnsweringMakesItReady(t *testing.T) {
	db := cleanCardProject(t)
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingOpenCard || pendings[0].About != "card-1" {
		t.Fatalf("before answer: %s %v, want draft with the open card", st, pendings)
	}
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara igual."); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusReady || len(pendings) != 0 {
		t.Fatalf("after answer: %s %v, want ready", st, pendings)
	}
}

func TestB15_LooseNumberIsAPendingOfTheSection(t *testing.T) {
	db := seedStepProject(t)
	applyRecorded(t, db, stepFile(t, "figure-datum-loose"))
	_, pendings := status(t, db, "sec-1")
	found := false
	for _, p := range pendings {
		found = found || (p.Kind == graph.PendingFigureWithoutDecision && p.About == "7")
	}
	if !found {
		t.Fatalf("pendings: %v, want figure_without_decision 7", pendings)
	}
}

// B-16, B-15: the other option leaves «texto con una decisión
// reemplazada» until an agent step on the section declares your
// decision among those it uses.
func TestB16_SupersededDecisionInTextClosesWithTheNextStepThatDeclaresIt(t *testing.T) {
	db := cleanCardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, "Porque el comité pidió conteos."); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingSupersededDecision || pendings[0].About != "dec-4" {
		t.Fatalf("after other: %s %v", st, pendings)
	}
	sec, _ := object(t, db, "sec-1")
	step := `{"triage_line":"Construir.","closing_risk":"none","note":"Reescribo con tu decisión.",
		"new_text":[{"section_id":"sec-1","base_version":` + itoa(sec.CurrentVersion) + `,"anchor":"Cierre.","operation":"replace","text":"Cierre con conteos crudos."}],
		"used_decisions":["dec-4"]}`
	if applied := applyRecorded(t, db, json.RawMessage(step)); !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusReady {
		t.Fatalf("after the step: %s %v, want ready", st, pendings)
	}
}

// B-15: a return without the fixed shape after its retry is a pending
// until the next valid step end.
func TestB15_MalformedOutputIsPendingUntilTheNextValidStep(t *testing.T) {
	db := cleanCardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara igual."); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	gate := cycle.NewGate(db)
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := gate.EndStep(context.Background(), "p1", "turn-2", attempt, json.RawMessage(`{"triage_line":"x"}`)); err != nil {
			t.Fatalf("EndStep: %v", err)
		}
	}
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingMalformedOutput {
		t.Fatalf("after the failed step: %s %v", st, pendings)
	}
	applyRecorded(t, db, json.RawMessage(`{"triage_line":"Construir.","closing_risk":"none","note":"Sigo."}`))
	if st, _ := status(t, db, "sec-1"); st != graph.StatusReady {
		t.Fatalf("after a valid step: %s, want ready", st)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
