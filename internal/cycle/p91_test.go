package cycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-91 (Eber 31-oct): an answer that arrives while endTurn prepares
// another defense (the judge takes ~30 s, the slot is held) still gets
// its own: the queued defense runs when the slot is released, with no
// further turn.
func TestP91_AnAnswerDuringEndTurnsDefenseStillGetsItsOwn(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"),
		judgeTextFixture(t, `{"audience_questions":[{"question":"¿Por qué ese límite?","section":"sec-2"},{"question":"¿Qué pasa con los estudios fuera del límite?","section":"sec-2"},{"question":"¿Quién fijó el límite?","section":"sec-2"}],"not_covered":[]}`))
	seedP86(t, db, "", "conclusion", 1, false) // sec-1, blocked by card-1
	seedP91SecondSection(t, db)                // sec-2, blocked by card-2
	ctx := context.Background()

	cap.judgeGate = make(chan struct{})
	release := cycle.HoldTurnForTest(runner, "p1")
	if _, err := runner.AnswerCard(ctx, "p1", "card-1", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard card-1: %v", err)
	}
	cycle.WaitAfterAnswersForTest(runner) // sec-1's defense queued for the turn's end
	ended := make(chan struct{})
	go func() { release(); close(ended) }() // endTurn prepares sec-1, held in the judge
	for deadline := time.Now().Add(5 * time.Second); judgeCalls(cap) < 1; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			close(cap.judgeGate)
			t.Fatal("endTurn never called the judge for sec-1")
		}
	}
	// Your answer while endTurn holds the slot: sec-2's defense queues.
	if _, err := runner.AnswerCard(ctx, "p1", "card-2", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard card-2: %v", err)
	}
	close(cap.judgeGate)
	<-ended
	cycle.WaitAfterAnswersForTest(runner)

	if len(cap.judges) != 2 || cap.judges[0].DefenseSection != "sec-1" || cap.judges[1].DefenseSection != "sec-2" {
		t.Fatalf("judge calls: %+v, want sec-1 then sec-2", cap.judges)
	}
	qs, about := audienceQuestions(t, db)
	var onSec2 int
	for _, q := range qs {
		if about[q.Label] == "sec-2" {
			onSec2++
		}
	}
	if onSec2 != 3 {
		t.Fatalf("audience questions about sec-2: %d, want 3", onSec2)
	}
}

func judgeCalls(c *captureProvider) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.judges)
}

// seedP91SecondSection adds sec-2, a concluding section blocked by
// card-2 on an agent decision used in it.
func seedP91SecondSection(t *testing.T, db *store.DB) {
	t.Helper()
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal"}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Límite de tamaño", "text": "Usamos un límite de tamaño fijo para todos los estudios.",
			"role": map[string]any{"value": "conclusion", "state": "confirmed"}, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
			"statement": "Fijar un límite de tamaño común.", "origin": "agent",
			"instead_of": "Usar el límite de cada estudio.",
		}, agent)
		if err != nil {
			return err
		}
		if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil,
			store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal"}); err != nil {
			return err
		}
		_, err = tx.CreateObject(store.ObjectCard, "open", map[string]any{"decision_id": dec.ID}, agent)
		return err
	})
	if err != nil {
		t.Fatalf("seed sec-2: %v", err)
	}
}
