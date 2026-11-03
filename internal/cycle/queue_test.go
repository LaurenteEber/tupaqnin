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

// A step that fails leaves your message in the history with nobody to read it: it
// says it was not sent, and «Reintentar» sends it now, once.
func TestB101_AfterAProviderFailureAQueuedMessageSaysItWasNotSentAndRetrySendsIt(t *testing.T) {
	down := errors.New("provider down")
	hold := &holdProvider{kind: provider.TurnStep, at: 1, fail: map[int]error{}}
	for n := 2; n <= 12; n++ {
		hold.fail[n] = down
	}
	_, runner, cap, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Usa la serie de 2024, por favor."
	if _, err := runner.StartTurn("p1", message); err != nil {
		t.Fatal(err)
	}
	queued, err := runner.QueuedMessages(ctx, "p1")
	if err != nil || len(queued) != 1 || queued[0].NotSent {
		t.Fatalf("during the step it only waits: %+v %v", queued, err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	queued, err = runner.QueuedMessages(ctx, "p1")
	if err != nil || len(queued) != 1 || queued[0].Text != message || !queued[0].NotSent {
		t.Fatalf("after the failure: %+v %v, want it marked not sent", queued, err)
	}
	if _, err := runner.RetryQueuedMessage(ctx, "p1", queued[0].N+1); !errors.Is(err, cycle.ErrMessageNotQueued) {
		t.Fatalf("retry of a message that does not wait: %v", err)
	}
	calls := len(cap.sent)
	hold.fail = nil // the provider is back
	turn, err := runner.RetryQueuedMessage(ctx, "p1", queued[0].N)
	if err != nil || turn == "" {
		t.Fatalf("retry: %q %v, want a turn to follow", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) <= calls || lastUserText(t, cap.sent[calls]) != message {
		t.Fatalf("the retried step's message: %d calls after %d", len(cap.sent), calls)
	}
	if queued, _ := runner.QueuedMessages(ctx, "p1"); len(queued) != 0 {
		t.Fatalf("still queued after the retry: %+v", queued)
	}
	if _, err := runner.RetryQueuedMessage(ctx, "p1", queued[0].N); !errors.Is(err, cycle.ErrMessageNotQueued) {
		t.Fatalf("retry after it entered: %v", err)
	}
}

// A retry sends every message waiting, in order, and the mark says how many.
func TestB101_ARetrySaysHowManyItSendsAndSendsThemInOrder(t *testing.T) {
	down := errors.New("provider down")
	hold := &holdProvider{kind: provider.TurnStep, at: 1, fail: map[int]error{}}
	for n := 2; n <= 12; n++ {
		hold.fail[n] = down
	}
	_, runner, cap, _ := newModeRunner(t, hold,
		textFixture(t, pendingFigureStep), loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	messages := []string{"Primero esto.", "Después esto.", "Y al final esto."}
	for _, m := range messages {
		if _, err := runner.StartTurn("p1", m); err != nil {
			t.Fatal(err)
		}
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	queued, err := runner.QueuedMessages(ctx, "p1")
	if err != nil || len(queued) != 3 {
		t.Fatalf("after the failure: %+v %v", queued, err)
	}
	for _, m := range queued {
		if !m.NotSent || m.RetrySends != 3 || m.NotSentCap {
			t.Fatalf("each mark says the three: %+v", m)
		}
	}
	calls := len(cap.sent)
	hold.fail = nil
	if _, err := runner.RetryQueuedMessage(ctx, "p1", queued[1].N); err != nil {
		t.Fatal(err)
	}
	cycle.WaitTurnsForTest(runner)
	sent := asJSON(t, cap.sent[calls])
	first, second, third := strings.Index(sent, messages[0]), strings.Index(sent, messages[1]), strings.Index(sent, messages[2])
	if first < 0 || second < first || third < second {
		t.Fatalf("the step's order: %d %d %d", first, second, third)
	}
	if left, _ := runner.QueuedMessages(ctx, "p1"); len(left) != 0 {
		t.Fatalf("still queued: %+v", left)
	}
}

// One message waiting: the plain retry says no count.
func TestB101_OneMessageWaitingRetrySendsOne(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1, fail: map[int]error{}}
	for n := 2; n <= 12; n++ {
		hold.fail[n] = errors.New("provider down")
	}
	_, runner, _, _ := newModeRunner(t, hold, textFixture(t, pendingFigureStep))
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	if _, err := runner.StartTurn("p1", "Sola."); err != nil {
		t.Fatal(err)
	}
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	queued, err := runner.QueuedMessages(context.Background(), "p1")
	if err != nil || len(queued) != 1 || queued[0].RetrySends != 1 {
		t.Fatalf("after the failure: %+v %v", queued, err)
	}
}

// B3-20, B-89: a queue stranded at a spend cap says so, and a retry over the cap
// stops at the cap: no call goes out and the message keeps waiting.
func TestB101_AQueueStrandedAtACapSaysSoAndARetryCallsNoModel(t *testing.T) {
	hold := &holdProvider{kind: provider.TurnStep, at: 1}
	db, runner, cap, _ := newModeRunner(t, hold,
		loadAppFixture(t, "04-principal-no-work.json"), loadAppFixture(t, "04-principal-no-work.json"))
	ctx := context.Background()
	if _, err := runner.StartTurn("p1", "Empieza."); err != nil {
		t.Fatal(err)
	}
	<-hold.entered
	const message = "Una cosa más."
	if _, err := runner.StartTurn("p1", message); err != nil {
		t.Fatal(err)
	}
	spend(t, db, "p1", 2.50) // the principal's reserve no longer fits
	close(hold.release)
	cycle.WaitTurnsForTest(runner)
	queued, err := runner.QueuedMessages(ctx, "p1")
	if err != nil || len(queued) != 1 || !queued[0].NotSent || !queued[0].NotSentCap {
		t.Fatalf("at the cap: %+v %v, want it marked not sent by the cap", queued, err)
	}
	calls, reports := len(cap.sent), len(cap.reports)
	spent, _ := db.ProjectSpend(ctx, "p1")
	turn, err := runner.RetryQueuedMessage(ctx, "p1", queued[0].N)
	if err != nil {
		t.Fatalf("retry: %q %v", turn, err)
	}
	cycle.WaitTurnsForTest(runner)
	if len(cap.sent) != calls || len(cap.reports) != reports {
		t.Fatalf("calls after the retry: %d steps, %d reports; want %d, %d", len(cap.sent), len(cap.reports), calls, reports)
	}
	if after, _ := db.ProjectSpend(ctx, "p1"); after != spent {
		t.Fatalf("spent %.4f, then %.4f: the stop spends nothing", spent, after)
	}
	left, err := runner.QueuedMessages(ctx, "p1")
	if err != nil || len(left) != 1 || left[0].Text != message || !left[0].NotSentCap {
		t.Fatalf("after the retry: %+v %v", left, err)
	}
}
