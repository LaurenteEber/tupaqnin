package cycle_test

import (
	"context"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// P-86 (Eber 31-oct, backlog-r2 2.6, fichas §8.2): the defense is also
// prepared when a section becomes ready by your own act, answering its
// last card or «por aclarar», with the conditions of the stop (an
// audience, B-38, or a concluding section) and not again when the
// section's text did not change (P-76).

const p86Reason = "Porque compara estudios con el mismo filtro."

// seedP86 seeds the defense project (seedDefense) and blocks sec-1 with
// cards open cards, each on an agent decision used in it, and, when
// question, one open «por aclarar» with a blocks edge to it.
func seedP86(t *testing.T, db *store.DB, audience, role string, cards int, question bool) {
	t.Helper()
	seedDefense(t, db, audience, role)
	ctx := context.Background()
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal"}
		for i := 0; i < cards; i++ {
			dec, err := tx.CreateObject(store.ObjectDecision, "agent", map[string]any{
				"statement": "Excluir los estudios sin filtro.", "origin": "agent",
				"instead_of": "Incluir todos los estudios.",
			}, agent)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil,
				store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal"}); err != nil {
				return err
			}
			if _, err := tx.CreateObject(store.ObjectCard, "open", map[string]any{"decision_id": dec.ID}, agent); err != nil {
				return err
			}
		}
		if !question {
			return nil
		}
		oq, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{
			"question": "¿El piloto D1 cuenta como estudio de campo?",
			"options": []any{
				map[string]any{"option": "Cuenta"},
				map[string]any{"option": "No cuenta"},
			},
		}, agent)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeBlocks, oq.ID, sec.ID, "", nil,
			store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal"})
		return err
	})
	if err != nil {
		t.Fatalf("seed P-86: %v", err)
	}
	if ok, err := graph.SectionClear(ctx, db, "p1", "sec-1"); err != nil || ok {
		t.Fatalf("sec-1 clear before your answer: %v %v, want blocked", ok, err)
	}
}

func audienceCount(t *testing.T, db *store.DB) int {
	t.Helper()
	qs, _ := audienceQuestions(t, db)
	return len(qs)
}

func TestP86_AnsweringTheLastCardPreparesTheDefense(t *testing.T) {
	fx := judgeFixture(t, "conclusion-section")
	want := recordedDefense(t, fx)
	db, runner, cap, _ := newCaptureRunner(t, fx)
	seedP86(t, db, "", "conclusion", 2, false)
	ctx := context.Background()

	// Not the last card: sec-1 is still blocked, no judge call.
	if _, err := runner.AnswerCard(ctx, "p1", "card-1", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard card-1: %v", err)
	}
	if len(cap.judges) != 0 {
		t.Fatalf("judge calls after card-1: %d, want 0 (card-2 still blocks sec-1)", len(cap.judges))
	}
	// The last card: sec-1 is ready by your act, the defense is prepared.
	if _, err := runner.AnswerCard(ctx, "p1", "card-2", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard card-2: %v", err)
	}
	if len(cap.judges) != 1 || cap.judges[0].DefenseSection != "sec-1" {
		t.Fatalf("judge calls: %+v, want one on sec-1", cap.judges)
	}
	qs, about := audienceQuestions(t, db)
	if len(qs) != len(want.AudienceQuestions)+len(want.NotCovered) {
		t.Fatalf("audience questions: %d, want %d", len(qs), len(want.AudienceQuestions)+len(want.NotCovered))
	}
	for _, q := range qs {
		if q.State != "open" || about[q.Label] != "sec-1" {
			t.Fatalf("%s: %s about %q, want open about sec-1", q.Label, q.State, about[q.Label])
		}
	}
	// It blocks nothing: the signoff of sec-1 opens.
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v %v, want opened with the audience questions open", open, err)
	}
}

func TestP86_AnsweringTheLastOpenQuestionPreparesTheDefense(t *testing.T) {
	fx := judgeFixture(t, "conclusion-section")
	db, runner, cap, _ := newCaptureRunner(t, fx)
	seedP86(t, db, "El comité de la planta", "other", 0, true)
	if _, err := runner.AnswerOpenQuestion(context.Background(), "p1", "oq-1", intp(1), p86Reason); err != nil {
		t.Fatalf("AnswerOpenQuestion: %v", err)
	}
	if len(cap.judges) != 1 || cap.judges[0].DefenseSection != "sec-1" {
		t.Fatalf("judge calls: %+v, want one on sec-1 (audience present, B-38)", cap.judges)
	}
	if audienceCount(t, db) == 0 {
		t.Fatal("no audience question opened")
	}
}

// B-38: no audience and a section that is not concluding: no judge call.
func TestP86_NoAudienceAndRoleOtherMakesNoJudgeCall(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	seedP86(t, db, "", "other", 1, false)
	if _, err := runner.AnswerCard(context.Background(), "p1", "card-1", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	if len(cap.judges) != 0 || audienceCount(t, db) != 0 {
		t.Fatalf("judge calls %d, audience questions %d; want none", len(cap.judges), audienceCount(t, db))
	}
}

// P-76: sec-1 becomes ready by your act a second time with the same
// text: no new judge call, and the first preparation's questions stay.
func TestP86_UnchangedTextSinceTheLastPreparationMakesNoNewCall(t *testing.T) {
	fx := judgeFixture(t, "conclusion-section")
	db, runner, cap, _ := newCaptureRunner(t, fx, judgeFixture(t, "conclusion-section"))
	seedP86(t, db, "", "conclusion", 1, false)
	ctx := context.Background()
	if _, err := runner.AnswerCard(ctx, "p1", "card-1", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard card-1: %v", err)
	}
	first := audienceCount(t, db)
	if len(cap.judges) != 1 || first == 0 {
		t.Fatalf("first preparation: %d judge calls, %d questions", len(cap.judges), first)
	}
	// A new open question blocks sec-1 again; answering it makes it ready
	// with the text unchanged.
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, "p1", "sec-1")
		if err != nil {
			return err
		}
		agent := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal"}
		oq, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{
			"question": "¿Cuenta D1?", "options": []any{map[string]any{"option": "Sí"}, map[string]any{"option": "No"}},
		}, agent)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeBlocks, oq.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	})
	if err != nil {
		t.Fatalf("seed oq: %v", err)
	}
	if _, err := runner.AnswerOpenQuestion(ctx, "p1", "oq-1", intp(0), p86Reason); err != nil {
		t.Fatalf("AnswerOpenQuestion: %v", err)
	}
	if len(cap.judges) != 1 || audienceCount(t, db) != first {
		t.Fatalf("judge calls %d, questions %d; want 1 and %d (same text, P-76)", len(cap.judges), audienceCount(t, db), first)
	}
}

// The defense after your answer never runs alongside a turn: with the
// project's turn slot taken it waits for the turn's end.
func TestP86_WithATurnRunningTheDefenseWaitsForItsEnd(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, judgeFixture(t, "conclusion-section"))
	seedP86(t, db, "", "conclusion", 1, false)
	ctx := context.Background()
	release := cycle.HoldTurnForTest(runner, "p1")
	if _, err := runner.AnswerCard(ctx, "p1", "card-1", cycle.OptionKeep, p86Reason); err != nil {
		t.Fatalf("AnswerCard: %v", err)
	}
	if len(cap.judges) != 0 {
		t.Fatalf("judge calls during the turn: %d, want 0", len(cap.judges))
	}
	release()
	if len(cap.judges) != 1 || audienceCount(t, db) == 0 {
		t.Fatalf("judge calls after the turn: %d, questions %d; want the defense prepared", len(cap.judges), audienceCount(t, db))
	}
}
