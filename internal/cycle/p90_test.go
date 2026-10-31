package cycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// P-90 (Eber 31-oct): the defense after your answer runs in the
// background. The answer returns while the judge is still answering;
// the audience questions are there once the defense ends.
func TestP90_TheAnswerReturnsBeforeTheDefenseEnds(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	cap.judgeGate = make(chan struct{})
	seedP86(t, db, "", "conclusion", 1, false)

	done := make(chan error, 1)
	go func() {
		_, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, p86Reason)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AnswerCard: %v", err)
		}
	case <-time.After(5 * time.Second):
		close(cap.judgeGate)
		t.Fatal("the answer waited for the judge")
	}
	if n := audienceCount(t, db); n != 0 {
		t.Fatalf("audience questions before the judge answers: %d, want 0", n)
	}
	close(cap.judgeGate)
	cycle.WaitAfterAnswersForTest(runner)
	if len(cap.judges) != 1 || audienceCount(t, db) == 0 {
		t.Fatalf("after the defense: %d judge calls, %d questions; want it prepared", len(cap.judges), audienceCount(t, db))
	}
}

// P-90: a defense that fails (the judge's output fails twice) never
// turns your saved answer into an error: the answer stands.
func TestP90_AFailedDefenseLeavesTheAnswerSaved(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	cap.judgeFails = true
	seedP86(t, db, "", "conclusion", 1, false)
	answer, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, p86Reason)
	if err != nil {
		t.Fatalf("AnswerCard: %v, want the answer saved", err)
	}
	cycle.WaitAfterAnswersForTest(runner)
	card, err := db.GetObject(context.Background(), "p1", "card-1")
	if err != nil || card.State == "open" {
		t.Fatalf("card-1: %+v %v, want answered (%+v)", card.State, err, answer)
	}
	if n := audienceCount(t, db); n != 0 {
		t.Fatalf("audience questions: %d, want none from a failed defense", n)
	}
}
