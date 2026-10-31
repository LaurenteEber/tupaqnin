package cycle_test

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// P-92 (Eber 31-oct): the answer says whether a defense is being
// prepared, and the runner reports it until the preparation ends, so
// the page knows when to stop asking.
func TestP92_TheAnswerSaysADefenseIsBeingPreparedUntilItEnds(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	cap.judgeGate = make(chan struct{})
	seedP86(t, db, "", "conclusion", 1, false)
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, p86Reason)
	if err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	if !answer.DefensePreparing || !runner.DefensePreparing("p1") {
		t.Fatalf("answer %+v, runner preparing %v; want both true", answer, runner.DefensePreparing("p1"))
	}
	close(cap.judgeGate)
	cycle.WaitAfterAnswersForTest(runner)
	if runner.DefensePreparing("p1") {
		t.Fatal("still preparing after the defense ended")
	}
	if audienceCount(t, db) == 0 {
		t.Fatal("no audience question: the preparation ended before writing them?")
	}
}

// P-92: an answer that leaves no defense to prepare (no audience, a
// section that does not conclude, B-38) says so.
func TestP92_AnAnswerWithNoDefenseToPrepareSaysNone(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	seedP86(t, db, "", "other", 1, false)
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, p86Reason)
	if err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	cycle.WaitAfterAnswersForTest(runner)
	if answer.DefensePreparing || runner.DefensePreparing("p1") || len(cap.judges) != 0 {
		t.Fatalf("answer %+v, preparing %v, judge calls %d; want none", answer, runner.DefensePreparing("p1"), len(cap.judges))
	}
}
