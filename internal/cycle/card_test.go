package cycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// cardProject: the figure step leaves dec-3 (the agent's, covering
// fig-1, used in sec-1) critical, with card-1 open.
func cardProject(t *testing.T) *store.DB {
	t.Helper()
	db := seedStepProject(t)
	applied := applyRecorded(t, db, stepFile(t, "figure-datum-loose"))
	if applied.Labels["new-1"] != "dec-3" || cardsOf(t, db)["dec-3"] != "open" {
		t.Fatalf("setup: labels %v, cards %v", applied.Labels, cardsOf(t, db))
	}
	return db
}

func TestE2_KeepLeavesTheDecisionConfirmedAndRecordsSeenAndTimed(t *testing.T) {
	db := cardProject(t)
	answer, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque compara estudios con el mismo filtro.")
	if err != nil || answer.Decision != "dec-3" {
		t.Fatalf("AnswerCard: %+v, %v", answer, err)
	}
	if dec, _ := object(t, db, "dec-3"); dec.State != "confirmed" {
		t.Fatalf("dec-3: %s, want confirmed", dec.State)
	}
	card, b := object(t, db, "card-1")
	if card.State != "answered" || b["chosen_option"] != "keep" || b["reason"] != "Porque compara estudios con el mismo filtro." {
		t.Fatalf("card-1: %s %v", card.State, b)
	}
	seen := eventsOfType(t, db, store.EventDecisionSeen)
	if len(seen) != 1 || eventData(t, seen[0])["seen_via"] != "card" {
		t.Fatalf("decision_seen: %v", seen)
	}
	if n := len(eventsOfType(t, db, store.EventActTimed)); n != 1 {
		t.Fatalf("act_timed: %d, want 1", n)
	}
}

func TestP2_AssentOrEmptyReasonLeavesTheCardOpen(t *testing.T) {
	db := cardProject(t)
	gate := cycle.NewGate(db)
	reasons := []string{"ok", "sí", "yes", "sure", ""}
	for _, reason := range reasons {
		if _, err := gate.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, reason); !errors.Is(err, cycle.ErrInvalidReason) {
			t.Fatalf("AnswerCard(%q): %v, want ErrInvalidReason", reason, err)
		}
	}
	if card, _ := object(t, db, "card-1"); card.State != "open" {
		t.Fatalf("card-1: %s, want open", card.State)
	}
	if dec, _ := object(t, db, "dec-3"); dec.State != "agent" {
		t.Fatalf("dec-3: %s, want agent", dec.State)
	}
	if n := len(eventsOfType(t, db, store.EventAnswerWithoutReason)); n != len(reasons) {
		t.Fatalf("answer_without_reason: %d, want %d", n, len(reasons))
	}
	if n := len(eventsOfType(t, db, store.EventActTimed)); n != len(reasons) {
		t.Fatalf("act_timed: %d, want %d (every answer)", n, len(reasons))
	}
}

// B-16: the other option gives birth to the person's decision, which
// supersedes the agent's and takes its covers and used_in edges.
func TestB16_OtherOptionCreatesYourDecisionAndSupersedesTheAgents(t *testing.T) {
	db := cardProject(t)
	answer, err := cycle.NewGate(db).AnswerCard(context.Background(), "p1", "card-1", cycle.OptionOther, "Porque el comité pidió conteos crudos.")
	if err != nil || answer.NewDecision != "dec-4" {
		t.Fatalf("AnswerCard: %+v, %v", answer, err)
	}
	old, ob := object(t, db, "dec-3")
	born, nb := object(t, db, "dec-4")
	if old.State != "superseded" || born.State != "human" {
		t.Fatalf("states: dec-3 %s, dec-4 %s", old.State, born.State)
	}
	if nb["statement"] != ob["instead_of"] || nb["instead_of"] != ob["statement"] ||
		nb["would_choose_if"] != ob["would_choose_if"] || nb["reason"] != "Porque el comité pidió conteos crudos." || nb["origin"] != "human" {
		t.Fatalf("born decision: %v (from %v)", nb, ob)
	}
	if got := edgeTypes(t, db, born.ID)[store.EdgeSupersedes]; len(got) != 1 || got[0] != old.ID {
		t.Fatalf("supersedes: %v", got)
	}
	fig, _ := object(t, db, "fig-1")
	if got := edgeTypes(t, db, fig.ID)[store.EdgeCovers]; len(got) != 1 || got[0] != born.ID {
		t.Fatalf("covers moved: %v", got)
	}
	sec, _ := object(t, db, "sec-1")
	if got := edgeTypes(t, db, born.ID)[store.EdgeUsedIn]; len(got) != 1 || got[0] != sec.ID {
		t.Fatalf("used_in moved: %v", got)
	}
	if got := edgeTypes(t, db, old.ID)[store.EdgeUsedIn]; len(got) != 0 {
		t.Fatalf("old decision still used_in: %v", got)
	}
	if got := cardsOf(t, db); len(got) != 1 || got["dec-3"] != "answered" {
		t.Fatalf("cards: %v, want only card-1 answered (no card for your decision)", got)
	}
}

func TestE3_AnsweringAClosedCardIsRefused(t *testing.T) {
	db := cardProject(t)
	gate := cycle.NewGate(db)
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Porque sí compara igual."); err != nil {
		t.Fatalf("first answer: %v", err)
	}
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, "Otra vez."); !errors.Is(err, cycle.ErrCardNotOpen) {
		t.Fatalf("second answer: %v, want ErrCardNotOpen", err)
	}
	if _, err := gate.AnswerCard(context.Background(), "p1", "card-1", "maybe", "Porque."); !errors.Is(err, cycle.ErrCardNotOpen) && !errors.Is(err, cycle.ErrInvalidOption) {
		t.Fatalf("bad option: %v", err)
	}
}
