package cycle_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// stopReasons lists the stop_reason of every step_ended of the principal.
func stopReasons(t *testing.T, db *store.DB) []string {
	t.Helper()
	var out []string
	for _, data := range stepEndedData(t, db) {
		reason, _ := data["stop_reason"].(string)
		out = append(out, reason)
	}
	return out
}

// P-99: two turns in a row on one fake-backed runner each give the
// principal the step recorded for it. The first turn asks for a round;
// the step after the digest uses the no-work recording, so the second
// turn's first step is the round recording again, not the leftover.
func TestP99_ASecondTurnOnTheSameRunnerGetsItsOwnStep(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, append(principalSteps(t, step), readerFixtures(t)...)...)
	for i, turn := range []string{"turn-1", "turn-2"} {
		if err := runner.RunTurn(context.Background(), "p1", turn, "Investiga."); err != nil {
			t.Fatalf("RunTurn %d: %v", i+1, err)
		}
		rounds, err := db.ListObjects(context.Background(), "p1", store.ObjectRound)
		if err != nil {
			t.Fatal(err)
		}
		if len(rounds) != i+1 {
			t.Fatalf("after turn %d: %d rounds, want %d; stop reasons %v", i+1, len(rounds), i+1, stopReasons(t, db))
		}
	}
}

// P-99: a step whose only work is a round does not stop the chain as
// «no work»: the principal goes on after the digest, and the stop ends in
// its report (B-37).
func TestP99_TheChainGoesOnAfterTheDigestOfARound(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, append(principalSteps(t, step), readerFixtures(t)...)...)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Investiga."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	got := stopReasons(t, db)
	if len(got) != 2 || got[0] != "" || got[1] != "no_work" {
		t.Fatalf("stop reasons %q, want the round step without one and then no_work", got)
	}
	if n := len(eventsOfType(t, db, store.EventStopReport)); n != 1 {
		t.Fatalf("stop reports: %d, want 1", n)
	}
}

// P-99: a principal that keeps asking for rounds cannot chain them: the
// chain goes on after its first round and stops after a second (B-65 caps
// the cost per round only).
func TestP99_TheChainDoesNotGoOnAfterASecondRound(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, append(
		principalSteps(t, step)[:1], readerFixtures(t)...)...)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Investiga."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	rounds, err := db.ListObjects(context.Background(), "p1", store.ObjectRound)
	if err != nil {
		t.Fatal(err)
	}
	if len(rounds) != 2 {
		t.Fatalf("rounds: %d, want 2 and then the stop; stop reasons %v", len(rounds), stopReasons(t, db))
	}
	// B-24: the step that stopped the chain says why.
	if got := stopReasons(t, db); len(got) != 2 || got[0] != "" || got[1] != "no_work" {
		t.Fatalf("stop reasons %q, want the first round's step without one and the second's no_work", got)
	}
	if n := len(eventsOfType(t, db, store.EventStopReport)); n != 1 {
		t.Fatalf("stop reports: %d, want 1", n)
	}
}

// P-99, B-24: a later turn's first round is again that chain's first: a
// round in an earlier turn does not make its step stop the chain.
func TestP99_ARoundInAnEarlierTurnDoesNotStopTheNextTurnsChain(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, append(principalSteps(t, step), readerFixtures(t)...)...)
	for _, turn := range []string{"turn-1", "turn-2"} {
		if err := runner.RunTurn(context.Background(), "p1", turn, "Investiga."); err != nil {
			t.Fatalf("RunTurn %s: %v", turn, err)
		}
	}
	want := []string{"", "no_work", "", "no_work"}
	if got := stopReasons(t, db); !slices.Equal(got, want) {
		t.Fatalf("stop reasons %q, want %q", got, want)
	}
}

// P-99: a request the app vetoed creates no round, so its step without
// text still stops the chain as «no work» (B-57).
func TestP99_AVetoedRoundStillStopsAsNoWork(t *testing.T) {
	step := discoverStep("risk_or_audit", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, principalSteps(t, step)...)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Investiga."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := stopReasons(t, db); len(got) != 1 || got[0] != "no_work" {
		t.Fatalf("stop reasons %q", got)
	}
	rounds, err := db.ListObjects(context.Background(), "p1", store.ObjectRound)
	if err != nil || len(rounds) != 0 {
		t.Fatalf("a vetoed request created rounds: %d, %v", len(rounds), err)
	}
}

// P-99, B-24: a round step on the chain's last allowed step says the limit
// stopped the chain.
func TestP99_ARoundOnTheLastAllowedStepSaysStepLimit(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, _, _ := newRoundRunner(t, false)
	applied, err := cycle.NewGate(db).ApplyChainStep(context.Background(), "p1", "turn-1", json.RawMessage(step), true)
	if err != nil || !applied.Applied || applied.Round == "" {
		t.Fatalf("apply: %+v, %v", applied, err)
	}
	if got := stopReasons(t, db); len(got) != 1 || got[0] != "step_limit" {
		t.Fatalf("stop reasons %q", got)
	}
}
