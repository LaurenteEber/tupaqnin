package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

func newCaptureRunner(t *testing.T, fixtures ...provider.Fixture) (*store.DB, *cycle.Runner, *captureProvider, *fakePublisher) {
	t.Helper()
	db, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Writer.ExecContext(context.Background(),
		`INSERT INTO project (id, created_at) VALUES ('p1', '2026-10-28T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	fake := provider.NewFake(fixtures...)
	fake.Recorder = db
	cap := &captureProvider{inner: fake}
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	pub := &fakePublisher{}
	return db, cycle.NewRunner(db, cap, count, pub), cap, pub
}

func lastUserText(t *testing.T, items []json.RawMessage) string {
	t.Helper()
	for i := len(items) - 1; i >= 0; i-- {
		var item struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(items[i], &item); err == nil && item.Role == "user" && len(item.Content) > 0 {
			return item.Content[0].Text
		}
	}
	return ""
}

func TestB17_ChainContinuesWithTheFixedMessageAndStopsAtAStepWithoutWork(t *testing.T) {
	_, runner, cap, pub := newCaptureRunner(t,
		loadAppFixture(t, "01-principal-valid.json"), loadAppFixture(t, "04-principal-no-work.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe la primera sección."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2 (the second step leaves no work)", len(cap.sent))
	}
	if got := lastUserText(t, cap.sent[1]); got != cycle.ContinueMessage {
		t.Fatalf("chained step message: %q, want %q", got, cycle.ContinueMessage)
	}
	if got := lastUserText(t, cap.sent[1]); strings.Contains(got, "primera sección") {
		t.Fatal("the chained step carries the person's text")
	}
	if step := pub.step(t); step["status"] != "accepted" || step["stop_reason"] != "no_work" {
		t.Fatalf("published step: %v", step)
	}
}

// P-43: the chained step tells the agent to write and not to wait for
// the person to confirm cases or roles (fichas §2: nothing blocks).
func TestP43_ChainedMessageSaysWriteTheNextSectionWithoutWaiting(t *testing.T) {
	if cycle.ContinueMessage != "Write the next section of the plan; do not wait for confirmations." {
		t.Fatalf("ContinueMessage: %q", cycle.ContinueMessage)
	}
}

func TestB17_ChainStopsAtFiveSteps(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t, loadAppFixture(t, "01-principal-valid.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != cycle.MaxChainSteps || cycle.MaxChainSteps != 5 {
		t.Fatalf("provider calls: %d, want 5", len(cap.sent))
	}
	if n := len(eventsOfType(t, db, store.EventStepEnded)); n != 5 {
		t.Fatalf("step_ended: %d, want 5", n)
	}
	if step := pub.step(t); step["stop_reason"] != "step_limit" {
		t.Fatalf("published step: %v", step)
	}
}

func TestB17_ChainStopsAtAStepPendingForTheAgent(t *testing.T) {
	db, runner, cap, pub := newCaptureRunner(t, loadAppFixture(t, "02-principal-invalid.json"))
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) != 2 {
		t.Fatalf("provider calls: %d, want 2 (one step and its retry)", len(cap.sent))
	}
	if step := pub.step(t); step["status"] != "agent_pending" || step["stop_reason"] != "agent_pending" {
		t.Fatalf("published step: %v", step)
	}
	if n := len(eventsOfType(t, db, store.EventOutputRetry)); n != 2 {
		t.Fatalf("output_retry: %d, want 2", n)
	}
}
