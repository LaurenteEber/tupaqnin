package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func closedRoundToLeaveOpen(t *testing.T) (*store.DB, *cycle.Runner) {
	t.Helper()
	db, runner, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, "other", false, "src-1") },
		[]string{projectPart("src-1")}, []string{readerOutput(t, projectFinding("fact", passageOne, "src-1"))},
		plainDigest(t, "partly"))
	return db, runner
}

// P-100, B-68: the scope «por aclarar» options carry their texts, so the
// choice reaches the principal as words and not as an empty «chosen».
func TestP100_ScopeOpenQuestionOptionsCarryTheB68Texts(t *testing.T) {
	db, runner := closedRoundToLeaveOpen(t)
	label, err := runner.LeaveRoundOpen(context.Background(), "p1", "rnd-1")
	if err != nil {
		t.Fatalf("LeaveRoundOpen: %v", err)
	}
	_, b := object(t, db, label)
	opts, _ := b["options"].([]any)
	if len(opts) != 2 {
		t.Fatalf("options: %v", b["options"])
	}
	want := map[string]string{
		cycle.ScopeOptionCompleteReview: "Complete the review with another round, with its cost",
		cycle.ScopeOptionDeclareLimit:   "Declare the limit in the text",
	}
	for _, o := range opts {
		opt := o.(map[string]any)
		if got := opt["option"]; got != want[opt["code"].(string)] {
			t.Fatalf("option %v: text %q", opt["code"], got)
		}
	}
}

// P-100: a version conflict leaving a round to clarify is reported as such,
// and its transaction leaves no «por aclarar» behind.
func TestP100_LeaveRoundOpenVersionConflictOpensNothing(t *testing.T) {
	db, runner := closedRoundToLeaveOpen(t)
	ctx := context.Background()
	t.Cleanup(cycle.SetBeforeLeaveOpenWrite(func() {
		round, err := db.GetObject(ctx, "p1", "rnd-1")
		if err != nil {
			t.Errorf("round: %v", err)
			return
		}
		var body map[string]any
		_ = json.Unmarshal(round.Body, &body)
		body["concurrent"] = true
		if err := db.Write(ctx, "p1", func(tx *store.Tx) error {
			_, err := tx.UpdateObject(round.ID, round.CurrentVersion, round.State, body,
				store.EventInput{Type: store.EventStateChanged, Actor: "app"})
			return err
		}); err != nil {
			t.Errorf("concurrent write: %v", err)
		}
	}))
	_, err := runner.LeaveRoundOpen(ctx, "p1", "rnd-1")
	if !errors.Is(err, store.ErrVersionConflict) {
		t.Fatalf("LeaveRoundOpen: %v, want ErrVersionConflict", err)
	}
	questions, err := db.ListObjects(ctx, "p1", store.ObjectOpenQuestion)
	if err != nil || len(questions) != 0 {
		t.Fatalf("open questions after the conflict: %d, %v", len(questions), err)
	}
}

// P-100: a round from your act on a project with no request is its own
// refusal, not an internal error, and it frees the turn slot.
func TestP100_StartRoundWithoutARequestIsRefusedAndFreesTheSlot(t *testing.T) {
	db, runner, _ := newTestRunner(t)
	t.Cleanup(runner.Close)
	for range 2 {
		if _, _, err := runner.StartRound("p1"); !errors.Is(err, cycle.ErrNoRequest) {
			t.Fatalf("StartRound: %v, want ErrNoRequest", err)
		}
	}
	rounds, err := db.ListObjects(context.Background(), "p1", store.ObjectRound)
	if err != nil || len(rounds) != 0 {
		t.Fatalf("rounds: %d, %v", len(rounds), err)
	}
}
