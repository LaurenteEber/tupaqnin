package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
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

// P-65, B-16, estados §2: the other option replaces the decision, and
// the sections that use it get their warning like any replacement
// (rule 4, «escrita con lo anterior», E-4); no pending of its own.
func TestP65_OtherOptionWarnsTheSectionsThatUseTheDecision(t *testing.T) {
	db := cleanCardProject(t)
	if _, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, "Porque el comité pidió conteos."); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	st, pendings := status(t, db, "sec-1")
	if st != graph.StatusStale || len(pendings) != 0 {
		t.Fatalf("after other: %s %v, want stale without pendings", st, pendings)
	}
	sec, _ := object(t, db, "sec-1")
	changes := graph.OpenChanges(sec)
	// The change names the replaced decision, as when the agent
	// replaces one (dec-4 is the one born from the card).
	if len(changes) != 1 || changes[0].Kind != "decision" || changes[0].About != "dec-3" {
		t.Fatalf("open changes: %v, want one about dec-3", changes)
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
		if _, err := gate.EndStep(context.Background(), "p1", "turn-2", attempt, json.RawMessage(`{"stretch": "build", "triage_line":"x"}`)); err != nil {
			t.Fatalf("EndStep: %v", err)
		}
	}
	if st, pendings := status(t, db, "sec-1"); st != graph.StatusDraft || len(pendings) != 1 || pendings[0].Kind != graph.PendingMalformedOutput {
		t.Fatalf("after the failed step: %s %v", st, pendings)
	}
	applyRecorded(t, db, json.RawMessage(`{"stretch": "build", "triage_line":"Construir.","closing_risk":"none","note":"Sigo."}`))
	if st, _ := status(t, db, "sec-1"); st != graph.StatusReady {
		t.Fatalf("after a valid step: %s, want ready", st)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// B-21: a section with no text stays draft, never ready, even with no
// pendings, so it is not offered for signing.
func TestB21_SectionWithoutTextStaysDraft(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Vacía", "text": "  \n",
			"role": map[string]any{"value": "other", "state": "confirmed"}},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if st, pendings := status(t, db, "sec-2"); st != graph.StatusDraft || len(pendings) != 0 {
		t.Fatalf("empty section: %s %v, want draft with no pendings", st, pendings)
	}
	if st, _ := status(t, db, "sec-1"); st != graph.StatusReady {
		t.Fatalf("section with text and no pendings: %s, want ready (previous behaviour)", st)
	}
}

// B-21: asking to sign a section without text opens nothing.
func TestB21_AskingToSignAnEmptySectionIsRefused(t *testing.T) {
	db := seedStepProject(t)
	ctx := context.Background()
	if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Vacía", "text": ""},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := cycle.NewGate(db).RequestSignoff(ctx, "p1", "sec-2"); !errors.Is(err, cycle.ErrSectionEmpty) {
		t.Fatalf("RequestSignoff on an empty section: %v, want ErrSectionEmpty", err)
	}
	if signs, _ := db.ListObjects(ctx, "p1", store.ObjectSignoff); len(signs) != 0 {
		t.Fatalf("signoffs: %d, want none", len(signs))
	}
}
