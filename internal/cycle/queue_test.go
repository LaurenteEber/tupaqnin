package cycle_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// B-101: in the normal mode a message during a step queues, without losing it,
// and enters the next step once and in literal.
func TestB101_InTheNormalModeAMessageQueuesAndEntersTheNextStep(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, cap, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Usa la serie de 2024, por favor."
	if turn, err := runner.StartTurn("p1", message); err != nil || turn != "" {
		t.Fatalf("StartTurn during a step: %q %v, want queued", turn, err)
	}
	if queued, err := runner.QueuedMessages(context.Background(), "p1"); err != nil || len(queued) != 1 || queued[0].Text != message {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if strings.Contains(asJSON(t, cap.sent[0]), message) {
		t.Fatal("the message reached the step already running")
	}
	if got := lastUserText(t, cap.sent[1]); got != message {
		t.Fatalf("next step's message: %q", got)
	}
	total := 0
	for _, sent := range cap.sent[2:] {
		total += strings.Count(lastUserText(t, sent), message)
	}
	if total != 0 {
		t.Fatalf("the message entered %d more times", total)
	}
	if queued, _ := runner.QueuedMessages(context.Background(), "p1"); len(queued) != 0 {
		t.Fatalf("still queued after it entered: %+v", queued)
	}
}

// B-101: a step with no work ends the chain with your message still queued: the
// release opens a new turn with it.
func TestB101_WhenTheChainEndsWithAMessageQueuedATurnOpensWithIt(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, cap, pub := newModeRunner(t, hold,
		loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Una cosa más."
	if turn, err := runner.StartTurn("p1", message); err != nil || turn != "" {
		t.Fatalf("StartTurn: %q %v", turn, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != 2 {
		t.Fatalf("step calls: %d, want the turn's and the message's", len(cap.sent))
	}
	if got := lastUserText(t, cap.sent[1]); got != message {
		t.Fatalf("the new turn's message: %q", got)
	}
	if pub.begun < 2 {
		t.Fatalf("turns begun: %d, want a new one for the message", pub.begun)
	}
}

// B-101: a message you remove never reaches the agent, and no turn opens for it.
func TestB101_RemovingAQueuedMessageDoesNotSendIt(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, cap, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Mejor no."
	if _, err := runner.StartTurn("p1", message); err != nil {
		t.Fatal(err)
	}
	queued, err := runner.QueuedMessages(context.Background(), "p1")
	if err != nil || len(queued) != 1 {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	if err := runner.WithdrawQueuedMessage(context.Background(), "p1", queued[0].N); err != nil {
		t.Fatal(err)
	}
	if err := runner.WithdrawQueuedMessage(context.Background(), "p1", queued[0].N); !errors.Is(err, cycle.ErrMessageNotQueued) {
		t.Fatalf("second removal: %v", err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	for i, sent := range cap.sent {
		if strings.Contains(asJSON(t, sent), message) {
			t.Fatalf("call %d got the removed message", i)
		}
	}
}

// B-101: an edited message enters with your new text only.
func TestB101_EditingAQueuedMessageSendsTheNewText(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, cap, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.StartTurn("p1", "Texto viejo."); err != nil {
		t.Fatal(err)
	}
	queued, _ := runner.QueuedMessages(context.Background(), "p1")
	if err := runner.EditQueuedMessage(context.Background(), "p1", queued[0].N, "Texto nuevo."); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	if got := lastUserText(t, cap.sent[1]); got != "Texto nuevo." {
		t.Fatalf("next step's message: %q", got)
	}
	for i, sent := range cap.sent {
		if strings.Contains(asJSON(t, sent), "Texto viejo.") {
			t.Fatalf("call %d got the old text", i)
		}
	}
}

// B-101: while a step's call is in flight its message can no longer be taken back,
// and it stops showing as queued.
func TestB101_AMessageEnteringTheRunningStepCannotBeRemoved(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	_, runner, _, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"))
	end := cycle.HoldTurnForTest(runner, "p1")
	if _, err := runner.StartTurn("p1", "Antes de empezar."); err != nil {
		t.Fatal(err)
	}
	queued, err := runner.QueuedMessages(context.Background(), "p1")
	if err != nil || len(queued) != 1 {
		t.Fatalf("queued: %+v %v", queued, err)
	}
	end() // the release opens a turn with it
	<-hold.entered
	if err := runner.WithdrawQueuedMessage(context.Background(), "p1", queued[0].N); !errors.Is(err, cycle.ErrMessageNotQueued) {
		t.Fatalf("removal with its step in flight: %v", err)
	}
	if err := runner.EditQueuedMessage(context.Background(), "p1", queued[0].N, "Otro."); !errors.Is(err, cycle.ErrMessageNotQueued) {
		t.Fatalf("edit with its step in flight: %v", err)
	}
	if now, _ := runner.QueuedMessages(context.Background(), "p1"); len(now) != 0 {
		t.Fatalf("still shows queued with its step in flight: %+v", now)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
}
